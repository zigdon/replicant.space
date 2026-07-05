package rest

import (
	"fmt"
	"time"

	"github.com/zigdon/rsp/models"
)

func durationFromSeconds(s float32) time.Duration {
	d, err := time.ParseDuration(fmt.Sprintf("%.2fs", s))
	if err != nil {
		log("Error parsing duration %v: %v", s, err)
	}
	return d
}

func fill[T []E, E models.Fillable](s []E) error {
	for _, e := range s {
		if err := e.Fill(); err != nil {
			return err
		}
	}
	return nil
}

func GetPrintQueueETA(dev *models.Device) (time.Duration, error) {
	if dev.Printing == nil && len(dev.PrintQueue) == 0 {
		return 0, nil
	}

	printTime := make(map[string]time.Duration)
	bps, err := Blueprints()
	if err != nil {
		return 0, fmt.Errorf("can't load blueprints: %v", err)
	}
	for _, bp := range bps.Blueprints {
		printTime[bp.DeviceType] = bp.PrintTime.Duration()
	}
	var res time.Duration
	if dev.Printing != nil {
		res += dev.Printing.Eta.Duration()
	}
	for _, q := range dev.PrintQueue {
		if l, ok := printTime[q.Type]; ok {
			res += l
		} else {
			return 0, fmt.Errorf("can't find print time for %q", q.Type)
		}
	}

	return res, nil
}
