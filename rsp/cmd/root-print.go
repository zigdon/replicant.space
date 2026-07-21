package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var rootPrintCmd = &cobra.Command{
	Use:   "print",
	Short: "Queue a print job at an autofactory",
	RunE:  rootPrint,
}

var rootPrintListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the current queue of all home autofactories",
	RunE:  rootPrintList,
}

func init() {
	rootCmd.AddCommand(rootPrintCmd)
	rootPrintCmd.Flags().String("home", "MENKUNT-2-L4", "Where can autofactories be found")
	rootPrintCmd.Flags().IntP("repeat", "r", 1, "How many copies should be printed")
	rootPrintCmd.Flags().StringP("controller", "c", "", "What controller should be assigned")
	rootPrintCmd.Flags().String("on_complete", "", "What commands to execute once done")

	rootPrintCmd.AddCommand(rootPrintListCmd)
	rootPrintListCmd.Flags().String("location", "", "Show only factories in this location")
	rootPrintListCmd.Flags().Bool("refresh", false, "When set, bypass the device cache")
}

func getHomeFactories(home string) ([]*models.CodeAlias, error) {
	factories, err := rest.Devices(map[string]string{"location": home, "device_type": "autofactory"})
	if err != nil {
		return nil, err
	}
	if len(factories) == 0 {
		return nil, fmt.Errorf("No factories found at %s", home)
	}
	log("%d factories found", len(factories))
	var printers []*models.CodeAlias
	for _, f := range factories {
		printers = append(printers, f.Code)
	}
	return printers, nil
}

func rootPrintList(cmd *cobra.Command, args []string) error {
	loc, _ := cmd.Flags().GetString("location")
	refresh, _ := cmd.Flags().GetBool("refresh")
	printers, err := rest.CachedDevices(
		map[string]string{"device_type": "autofactory"}, !refresh)
	if err != nil {
		return err
	}
	type pq struct {
		location   models.LocationID
		code       *models.CodeAlias
		deviceType string
		tags       []string
		pos        int
		eta        time.Time
		missing    map[string]int
	}
	times := make(map[string]time.Duration)
	var queue []pq
	totalMissing := make(map[string]int)
	for _, info := range printers {
		if loc != "" && string(info.Location) != loc {
			continue
		}
		if info.Status == "waiting_for_resources" {
			info, err = rest.RefreshDeviceInfo(info.Code)
			if err != nil {
				return err
			}
		}
		if info.Status == "waiting_for_resources" {
			missing := make(map[string]int)
			for k, v := range info.WaitingFor.Resources {
				if v.Have < v.Need {
					missing[k] = v.Need - v.Have
				}
				totalMissing[k] += missing[k]
			}
			for k, v := range info.WaitingFor.Components {
				if v.Have < v.Need {
					missing[k] = v.Need - v.Have
				}
				totalMissing[k] += missing[k]
			}
			queue = append(queue, pq{
				location:   info.Location,
				code:       info.Code,
				deviceType: "Waiting for resources",
				pos:        -1,
				missing:    missing,
			})
		} else if info.Printing != nil {
			queue = append(queue, pq{
				location:   info.Location,
				code:       info.Code,
				deviceType: info.Printing.DeviceType,
				tags:       info.Printing.Tags,
				pos:        -1,
				eta:        info.Printing.Completes.Time(),
			})
			times[info.Code.Alias()] += info.Printing.Eta.Duration()
		}
		for i, q := range info.PrintQueue {
			if info.Status == "waiting_for_resources" && i == 0 {
				continue
			}
			bp := getBP(q.Type)
			times[info.Code.Alias()] += bp.PrintTime.Duration()
			queue = append(queue, pq{
				code:       info.Code,
				deviceType: q.Type,
				tags:       q.Tags,
				pos:        i,
				eta:        time.Now().Add(bp.PrintTime.Duration()).Add(times[info.Code.Alias()]),
			})
		}
	}

	slices.SortFunc(queue, func(a, b pq) int {
		return cmp.Compare(a.eta.Unix(), b.eta.Unix())
	})
	var data [][]string
	for _, q := range queue {
		var pos string
		if q.pos < 0 {
			pos = "printing"
		} else {
			pos = d(q.pos)
		}
		data = append(data, []string{
			string(q.location), q.deviceType, list(q.tags), q.code.Alias(), pos, t(q.eta), rm(q.missing),
		})
	}
	printTable([]string{"Location", "Type", "Tags", "Factory", "Position", "ETA", "Missing"}, data)

	if len(totalMissing) > 0 {
		data = [][]string{}
		for k, v := range totalMissing {
			data = append(data, []string{k, d(v)})
		}
		printTable([]string{"Missing", "Quantity"}, data)
	}

	return nil
}

func rootPrint(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("Usage: rsp print <device> [-r <copies>]")
	}
	name := args[0]
	if full := db.GetTypeForPrefix(args[0]); full != "" {
		name = full
	}

	home, _ := cmd.Flags().GetString("home")
	printers, err := getHomeFactories(home)
	if err != nil {
		return err
	}

	// Figure out what dependencies are missing
	inventory := make(map[string]int)
	available := make(map[string]int)
	pending := make(map[string]int)
	loc, err := rest.Location(home)
	if err != nil {
		return err
	}
	// Check what's already available
	for _, i := range loc.Inventory {
		inventory[i.ResourceType] = int(i.Quantity)
	}
	// Check what's already being printed
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

	copies, _ := cmd.Flags().GetInt("repeat")
	var data [][]string
	bp := getBP(name)
	log("Print time, per copy: %s", bp.PrintTime.Duration())
	for k, v := range bp.Resources {
		data = append(data, []string{k, d(v), d(v * copies), d(inventory[k]), ""})
	}
	for k, v := range bp.Components {
		data = append(data, []string{k, d(v), d(v * copies), d(inventory[k]), d(pending[k])})
	}
	printTable([]string{"Ingredient", "Per copy", "Total needed", "Available", "Queued"}, data)

	// Simulate printing, so we can figure out what we actually need
	type batch struct {
		name string
		qty  int
	}
	var toPrint []batch
	var simulate func(string, int) error
	simulate = func(name string, qty int) error {
		if qty <= 0 {
			return nil
		}
		toPrint = append(toPrint, batch{name: name, qty: qty})
		bp := getBP(name)
		log("Simulating printing of %d %s", qty, name)
		for r, q := range bp.Resources {
			log("... need %d x %s", q*qty, r)
			if inventory[r] < q*qty {
				return fmt.Errorf("Not enough %s for printing %d %s: have %d, need %d",
					r, qty, name, inventory[r], q*qty)
			}
			inventory[r] -= q * qty
		}
		for c, q := range bp.Components {
			log("... need %d x %s", q*qty, c)
			missing := q*qty - inventory[c]
			if missing > 0 {
				if err := simulate(c, missing); err != nil {
					return err
				}
			}
			inventory[c] -= q * qty
		}
		return nil
	}
	if err := simulate(name, copies); err != nil {
		return fmt.Errorf("Printing simulation failed: %v", err)
	}
	log("Print queue:")
	slices.Reverse(toPrint)
	for _, p := range toPrint {
		log("  %d x %s", p.qty, p.name)
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
		info, err := getInfo(p)
		if err != nil {
			return err
		}
		r.avail = info.QueueSize - len(info.PrintQueue)
		slots += r.avail
		if info.Printing != nil {
			slots -= 1
		}
		r.delay, err = rest.GetPrintQueueETA(info)
		if err != nil {
			return err
		}
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
			pl.eta = pl.eta + getBP(next.name).PrintTime.Duration()
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
			log("Ran out of print slots, %d copies remaining", copies)
			break
		}
	}

	controller, _ := cmd.Flags().GetString("controller")
	onComplete, _ := cmd.Flags().GetString("on_complete")
	cfg := map[string]any{
		"device_type": bp.DeviceType,
	}
	if controller != "" {
		cfg["controller"] = controller
	}
	if onComplete != "" {
		cfg["oncomplete"] = onComplete
	}

	slices.SortFunc(printers, func(a, b *models.CodeAlias) int {
		return cmp.Compare(a.Num(), b.Num())
	})

	data = [][]string{}
	for _, p := range printers {
		pl, ok := plan[p.Alias()]
		if !ok || len(pl.toQueue) == 0 {
			continue
		}
		var delay string
		if pl.delay > 0 {
			delay = dt(pl.delay)
		}
		data = append(data, []string{
			p.Alias(), countList(pl.toQueue), delay, dt(pl.eta),
		})
		for _, tq := range pl.toQueue {
			if tq == "" {
				continue
			}
			log("printing %q on %q", tq, p.Alias())
			if tq == name {
				_, err = rest.DeviceCommand[models.CommandResp](p, "enqueue_print", cfg)
			} else {
				_, err = rest.DeviceCommand[models.CommandResp](p, "enqueue_print", map[string]any{
					"device_type": tq,
				})
			}
			if err != nil {
				return err
			}
		}
	}
	printTable([]string{"Factory", "Copies", "Delay", "ETA"}, data)
	return nil
}
