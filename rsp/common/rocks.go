package common

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/gammazero/workerpool"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var rockTS time.Time
var rocks []*models.Object

func GetRocks() ([]*models.Object, error) {
	if time.Since(rockTS) < 5*time.Minute && rocks != nil {
		return rocks, nil
	}
	Log("Reading logs from hubs...")
	shs, err := rest.Devices(map[string]string{"device_type": "system_hub"})
	if err != nil {
		return nil, err
	}
	Log("Reading logs from beacons...")
	beacons, err := rest.Devices(map[string]string{"device_type": "ftl_beacon"})
	if err != nil {
		return nil, err
	}
	logs := append(shs, beacons...)

	var errs []error
	var objs []*models.Object
	Log("Loading logs...")
	var mu sync.Mutex
	stat := make(map[string]int)
	wp := workerpool.New(10)
	for _, sh := range logs {
		wp.Submit(func() {
			fmt.Print(".")
			logs, err := rest.DeviceLogs(sh.Code, -1)
			if err != nil {
				errs = append(errs, fmt.Errorf("Error getting logs from %q: %v", sh.Code.Alias(), err))
				return
			}
			for _, e := range logs.Events {
				if e.EventType != "system_object_detected" {
					continue
				}
				etaStr := e.Payload["impact_eta"].(string)
				eta, err := time.Parse("2006-01-02T15:04:05.999999", etaStr)
				if err != nil {
					errs = append(errs, fmt.Errorf("Error parsing timestamp %q: %v", etaStr, err))
					return
				}
				id := e.Payload["object_designation"].(string)
				if time.Now().After(eta) {
					continue
				}
				info, err := rest.Location(id)
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("Can't get info for %q: %v", id, err))
					mu.Unlock()
					continue
				}
				if info.Object == nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("Info for %q does not include object: %v", id, info))
					mu.Unlock()
					continue
				}
				mu.Lock()
				objs = append(objs, info.Object)
				stat[info.Object.Status]++
				mu.Unlock()
			}
		})
	}
	wp.StopWait()
	fmt.Printf(" %d rocks: %v\n", len(objs), stat)

	slices.SortFunc(objs, func(a, b *models.Object) int {
		return cmp.Compare(a.Designation.Star(), b.Designation.Star())
	})

	rocks = objs
	rockTS = time.Now()
	return objs, errors.Join(errs...)
}
