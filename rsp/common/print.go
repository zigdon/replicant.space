package common

import (
	"cmp"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

type PrintPlanRec struct {
	Queued []string
	ETA    time.Time
}

type PrintSetEntry struct {
	Name   string
	Qty    int
	Config map[string]any
}

type PrintPlan struct {
	Location string
	Set      []PrintSetEntry
	Qty      int
	Device   string
	ETA      time.Time
	Printers map[*models.CodeAlias]*PrintPlanRec
}

func CheckQueue(where, tag, devType string, qty int) (int, time.Time) {
	eta := time.Now()
	Log("Checking %s print queue for %d × %q (tagged %q)", where, qty, devType, tag)
	printers, err := GetFilteredDevices(
		[]string{"autofactory"}, []string{where}, []string{"waiting_for_resources", "printing"})
	if err != nil {
		Log("Failed to fetch home autofactories: %v", err)
		return 0, eta
	}
	var found int
	later := func(a, b time.Time) time.Time {
		if a.After(b) {
			return a
		}
		return b
	}
	for _, p := range printers {
		info, err := rest.DeviceInfo(p)
		if err != nil {
			Log("Failed to fetch details for %q: %v", p, err)
			return 0, eta
		}
		if info.Printing != nil && info.Printing.DeviceType == devType &&
			slices.Contains(info.Printing.Tags, tag) {
			found++
			eta = later(eta, info.Printing.Completes.Time())
		}
		for _, pq := range info.PrintQueue {
			if pq.Type != devType {
				continue
			}
			if !slices.Contains(pq.Tags, tag) {
				continue
			}
			// It'll be _at least_ however much is left in the current print + print time
			if info.Printing != nil {
				eta = later(eta, info.Printing.Completes.Time().Add(GetBP(pq.Type).PrintTime.Duration()))
			} else {
				eta = later(eta, time.Now().Add(GetBP(pq.Type).PrintTime.Duration()))
			}
			found++
		}
		if found >= qty {
			break
		}
	}

	Log("... found %d %q (%q)", found, devType, tag)
	return found, eta
}

func Print(where, name string, qty int, useInventory, dryRun bool, cfg map[string]any) (*PrintPlan, error) {
	plan, err := PrintSet(where, []PrintSetEntry{{Name: name, Qty: qty, Config: cfg}}, useInventory, dryRun)
	plan.Device = name
	plan.Qty = qty

	return plan, err
}

func PrintSet(where string, set []PrintSetEntry, useInventory, dryRun bool) (*PrintPlan, error) {
	empty := true
	for _, s := range set {
		if s.Qty > 0 {
			empty = false
			break
		}
	}
	if empty {
		Log("Nothing to print")
		return nil, nil
	}
	Log("***********************")
	Log("Printing set at %s:", where)
	bps := make(map[string]*models.Blueprint)
	for _, s := range set {
		bps[s.Name] = GetBP(s.Name)
		Log("  - %d x %s %v (%s per copy)", s.Qty, s.Name, s.Config, bps[s.Name].PrintTime)
	}
	pPlan := &PrintPlan{
		Location: where,
		Set:      set,
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

	var lines []string
	totals := make(map[string]int)
	for _, s := range set {
		for k, v := range bps[s.Name].Resources {
			if !slices.Contains(lines, k) {
				lines = append(lines, k)
			}
			totals[k] += v * s.Qty
		}
		for k, v := range bps[s.Name].Components {
			bps[k] = GetBP(k)
			if !slices.Contains(lines, k) {
				lines = append(lines, k)
			}
			totals[k] += v * s.Qty
		}
	}

	var data [][]any
	for _, l := range lines {
		data = append(data, []any{l, totals[l], inventory[l], pending[l]})
	}
	PrintTable([]string{"Ingredient", "Needed", "Available", "Queued"}, data)

	// Simulate printing, so we can figure out what we actually need
	type batch struct {
		name string
		qty  int
		wave int
		cfg  map[string]any
	}
	var toPrint []batch
	var simulate func(string, int, int, map[string]any) error
	printCost := make(map[string]int)
	simulate = func(name string, qty, wave int, cfg map[string]any) error {
		if qty <= 0 {
			return nil
		}
		toPrint = append(toPrint, batch{name: name, qty: qty, wave: wave, cfg: cfg})
		bp := GetBP(name)
		if bp == nil {
			return fmt.Errorf("Blueprint not available for %q", name)
		}
		Log("Simulating printing of %d %s", qty, name)
		for r, q := range bp.Resources {
			Log("... need %d × %s", q*qty, r)
			printCost[r] += q * qty
			if inventory[r] < q*qty {
				return fmt.Errorf("Not enough %s for printing %d %s at %s: have %d, need %d",
					r, qty, name, where, inventory[r], q*qty)
			}
			inventory[r] -= q * qty
		}
		for c, q := range bp.Components {
			Log("... need %d × %s", q*qty, c)
			missing := q * qty
			if useInventory {
				missing -= inventory[c]
			}
			if missing > 0 {
				if err := simulate(c, missing, wave+1, map[string]any{"device_type": c}); err != nil {
					return err
				}
			}
			inventory[c] -= q * qty
		}
		return nil
	}
	var errs []error
	for _, s := range set {
		if s.Config == nil {
			s.Config = make(map[string]any)
		}
		s.Config["device_type"] = s.Name
		errs = append(errs, simulate(s.Name, s.Qty, 0, s.Config))
	}
	if err := errors.Join(errs...); err != nil {
		return pPlan, fmt.Errorf("Printing simulation failed: %v", err)
	}
	Log("Print queue:")
	// Sort the print order by waves, so dependencies are printed before things that need them.
	slices.SortFunc(toPrint, func(a, b batch) int {
		return cmp.Or(
			cmp.Compare(b.wave, a.wave),
			cmp.Compare(a.name, b.name),
		)
	})
	for _, p := range toPrint {
		Log("  %d × %s", p.qty, p.name)
	}
	Log("Total cost:")
	for k, v := range printCost {
		Log("  %d × %s", v, k)
	}

	// Check each printer for available print slots, and eta
	var slots int
	type rec struct {
		delay   time.Duration
		eta     time.Duration
		toQueue []string
		cfgs    []map[string]any
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
			cfg := next.cfg
			cfg["device_type"] = bps[next.name].DeviceType
			pl.cfgs = append(pl.cfgs, next.cfg)
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
			Log("Could not find print slots for:")
			for _, tp := range toPrint {
				Log("  %d x %s", tp.qty, tp.name)
			}
			return pPlan, fmt.Errorf("Ran out of print slots")
		}
	}

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
		if pPlan.Printers[p].ETA.After(pPlan.ETA) {
			pPlan.ETA = pPlan.Printers[p].ETA
		}
		data = append(data, []any{
			p, CountList(pl.toQueue), pl.delay, pl.eta,
		})
		for n, tq := range pl.toQueue {
			if tq == "" {
				continue
			}
			if dryRun {
				Log("would print %q on %q %v", tq, p.Alias(), pl.cfgs[n])
				continue
			}
			_, err = rest.DeviceCommand[models.CommandResp](p, "enqueue_print", pl.cfgs[n])
			if err != nil {
				return pPlan, fmt.Errorf("Failed to queue %q at %s: %v", pl.cfgs[n]["device_type"], p, err)
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

	// Randomize the order of the printers, to make it less likely that we
	// queue 2 devices on the same one due to caching.
	rand.Shuffle(len(printers), func(i, j int) {
		printers[i], printers[j] = printers[j], printers[i]
	})

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

	return printers[0], nil
}
