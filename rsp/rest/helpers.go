package rest

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/zigdon/rsp/models"
)

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
	bps, err := Blueprints(false)
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

func FindPrinter(printers []*models.CodeAlias, extra map[string]time.Duration) (*models.CodeAlias, error) {
	// Check the queue for each potential printer. If there is an idle printer,
	// use that. Otherwise, pick the one with the shortest queue, by remaining
	// print time.
	info := make(map[*models.CodeAlias]*models.Device)
	log("Printers:")
	for _, p := range printers {
		i, err := DeviceInfo(p)
		if err != nil {
			return nil, fmt.Errorf("can't get device info for %q: %v", p, err)
		}
		info[p] = i
		log("  %s: %s (%s already queued)", p.Alias(), i.Type, extra[p.String()])
	}

	// Calculate the queue length for each printer
	queue := make(map[*models.CodeAlias]time.Duration)
	for _, p := range printers {
		eta, err := GetPrintQueueETA(info[p])
		if err != nil {
			return nil, fmt.Errorf("error getting print queue for %q: %v", p, err)
		}
		queue[p] = eta + extra[p.String()]
	}
	if len(queue) == 0 {
		return nil, fmt.Errorf("No available printer found")
	}
	slices.SortFunc(printers, func(a, b *models.CodeAlias) int {
		ta, _ := queue[a]
		tb, _ := queue[b]
		return cmp.Compare(ta, tb)
	})
	for _, p := range printers {
		log("%s: %s", p.Alias(), queue[p])
	}

	return printers[0], nil
}
