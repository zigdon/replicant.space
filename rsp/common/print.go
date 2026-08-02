package common

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

type PrintPlanRec struct {
	Queued []string
	ETA    time.Time
}

type PrintPlan struct {
	Location string
	Qty      int
	Device   string
	ETA      time.Time
	Printers map[*models.CodeAlias]*PrintPlanRec
}

func Print(where, name string, qty int, useInventory, dryRun bool, cfg map[string]any) (*PrintPlan, error) {
	bp := GetBP(name)
	Log("***********************")
	Log("Printing %d of %s at %s", qty, name, where)
	Log("Print time, per copy: %s", bp.PrintTime.Duration())
	pPlan := &PrintPlan{
		Location: where,
		Qty:      qty,
		Device:   name,
		Printers: make(map[*models.CodeAlias]*PrintPlanRec),
	}

	printers, err := GetFilteredDevices([]string{"autofactory"}, []string{where}, []string{"idle", "printing"})
	if err != nil {
		return pPlan, err
	}

	// Figure out what dependencies are missing
	inventory := make(map[string]int)
	available := make(map[string]int)
	pending := make(map[string]int)
	loc, err := rest.Location(where)
	if err != nil {
		return pPlan, err
	}
	// Check what resources are already available
	for _, i := range loc.Inventory {
		inventory[i.ResourceType] = int(i.Quantity)
	}
	// Check what devices are there, or are being printed
	for _, d := range loc.Devices {
		available[d.Type]++
		if d.Type == "autofactory" {
			if d.Printing != nil {
				pending[d.Printing.DeviceType]++
			}
			for _, p := range d.PrintQueue {
				pending[p.Type]++
			}
		}
	}

	var data [][]any
	for k, v := range bp.Resources {
		data = append(data, []any{k, v, v * qty, inventory[k], ""})
	}
	for k, v := range bp.Components {
		data = append(data, []any{k, v, v * qty, inventory[k], pending[k]})
	}
	PrintTable([]string{"Ingredient", "Per copy", "Total needed", "Available", "Queued"}, data)

	// Simulate printing, so we can figure out what we actually need
	type batch struct {
		name string
		qty  int
	}
	var toPrint []batch
	var simulate func(string, int) error
	printCost := make(map[string]int)
	simulate = func(name string, qty int) error {
		if qty <= 0 {
			return nil
		}
		toPrint = append(toPrint, batch{name: name, qty: qty})
		bp := GetBP(name)
		Log("Simulating printing of %d %s", qty, name)
		for r, q := range bp.Resources {
			Log("... need %d x %s", q*qty, r)
			printCost[r] += q * qty
			if inventory[r] < q*qty {
				return fmt.Errorf("Not enough %s for printing %d %s: have %d, need %d",
					r, qty, name, inventory[r], q*qty)
			}
			inventory[r] -= q * qty
		}
		for c, q := range bp.Components {
			Log("... need %d x %s", q*qty, c)
			missing := q * qty
			if useInventory {
				missing -= inventory[c]
			}
			if missing > 0 {
				if err := simulate(c, missing); err != nil {
					return err
				}
			}
			inventory[c] -= q * qty
		}
		return nil
	}
	if err := simulate(name, qty); err != nil {
		return pPlan, fmt.Errorf("Printing simulation failed: %v", err)
	}
	Log("Print queue:")
	slices.Reverse(toPrint)
	for _, p := range toPrint {
		Log("  %d x %s", p.qty, p.name)
	}
	Log("Total cost:")
	for k, v := range printCost {
		Log("  %d x %s", v, k)
	}

	// Check each printer for available print slots, and eta
	var slots int
	type rec struct {
		delay   time.Duration
		eta     time.Duration
		toQueue []string
		avail   int
	}
	plan := make(map[string]*rec)
	for _, p := range printers {
		r := new(rec)
		info, err := rest.DeviceInfo(p)
		if err != nil {
			return pPlan, err
		}
		r.avail = info.QueueSize - len(info.PrintQueue)
		slots += r.avail
		if info.Printing != nil {
			slots -= 1
		}
		r.delay = GetPrintQueueETA(info)
		r.eta = r.delay
		plan[p.Alias()] = r
	}

	for len(toPrint) > 0 {
		// Sort the printers by next available
		slices.SortFunc(printers, func(a, b *models.CodeAlias) int {
			return cmp.Compare(plan[a.Alias()].eta, plan[b.Alias()].eta)
		})
		var found bool
		next := toPrint[0]
		for _, p := range printers {
			pl := plan[p.Alias()]
			if pl.avail == 0 {
				continue
			}
			// Add the next print
			pl.toQueue = append(pl.toQueue, next.name)
			pl.eta = pl.eta + GetBP(next.name).PrintTime.Duration()
			plan[p.Alias()] = pl
			found = true
			break
		}
		if found {
			next.qty--
			if next.qty > 0 {
				toPrint[0] = next
			} else {
				toPrint = toPrint[1:]
			}
		} else {
			Log("Ran out of print slots, %d copies remaining", qty)
			break
		}
	}

	cfg["device_type"] = bp.DeviceType

	slices.SortFunc(printers, func(a, b *models.CodeAlias) int {
		return cmp.Compare(a.Num(), b.Num())
	})

	data = [][]any{}
	for _, p := range printers {
		pl, ok := plan[p.Alias()]
		if !ok || len(pl.toQueue) == 0 {
			continue
		}
		pPlan.Printers[p] = &PrintPlanRec{
			ETA:    time.Now().Add(pl.eta),
			Queued: pl.toQueue,
		}
		data = append(data, []any{
			p, CountList(pl.toQueue), pl.delay, pl.eta,
		})
		for _, tq := range pl.toQueue {
			if tq == "" {
				continue
			}
			if dryRun {
				Log("would print %q on %q", tq, p.Alias())
				continue
			}
			if tq == name {
				_, err = rest.DeviceCommand[models.CommandResp](p, "enqueue_print", cfg)
			} else {
				_, err = rest.DeviceCommand[models.CommandResp](p, "enqueue_print", map[string]any{
					"device_type": tq,
				})
			}
			if err != nil {
				return pPlan, err
			}
		}
	}
	PrintTable([]string{"Factory", "Copies", "Delay", "ETA"}, data)
	return pPlan, nil
}

func FindPrinter(printers []*models.CodeAlias, extra map[string]time.Duration) (*models.CodeAlias, error) {
	// Check the queue for each potential printer. If there is an idle printer,
	// use that. Otherwise, pick the one with the shortest queue, by remaining
	// print time.
	info := make(map[*models.CodeAlias]*models.Device)
	for _, p := range printers {
		i, err := rest.DeviceInfo(p)
		if err != nil {
			return nil, fmt.Errorf("can't get device info for %q: %v", p, err)
		}
		info[p] = i
	}

	// Calculate the queue length for each printer
	queue := make(map[*models.CodeAlias]time.Duration)
	full := make(map[string]bool)
	for _, p := range printers {
		if len(info[p].PrintQueue) >= info[p].QueueSize {
			Log("%s has a full queue", p.Alias())
			full[p.Alias()] = true
			continue
		}
		eta := GetPrintQueueETA(info[p])
		queue[p] = eta + extra[p.String()]
	}
	for k := range queue {
		if full[k.Alias()] {
			delete(queue, k)
		}
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
		Log("%s: %s", p.Alias(), queue[p])
	}

	return printers[0], nil
}
