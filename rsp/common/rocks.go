package common

import (
	"cmp"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var rockTS time.Time
var rocks []*models.Object

func GetRocks() ([]*models.Object, error) {
	if time.Since(rockTS) < 5*time.Minute && rocks != nil {
		return rocks, nil
	}
	Log("Finding hubs...")
	shs, err := rest.Devices(map[string]string{"device_type": "system_hub"})
	if err != nil {
		return nil, err
	}

	var errs []error
	var objs []*models.Object
	Log("Loading logs...")
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, sh := range shs {
		wg.Go(func() {
			fmt.Print(".")
			logs, err := rest.DeviceLogs(sh.Code, true, 0, 50)
			if err != nil {
				errs = append(errs, fmt.Errorf("Error getting logs from %q: %v", sh.Code.Alias(), err))
				return
			}
			for _, e := range logs.Events {
				if e.EventType != "system_object_detected" {
					continue
				}
				id := e.Payload["object_designation"].(string)
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
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	fmt.Printf(" %d rocks\n", len(objs))

	slices.SortFunc(objs, func(a, b *models.Object) int {
		return cmp.Compare(a.Designation.Star(), b.Designation.Star())
	})

	rocks = objs
	rockTS = time.Now()
	return objs, nil
}
