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
	printers, err := rest.Devices(map[string]string{"device_type": "autofactory"})
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
	for _, info := range printers {
		if loc != "" && string(info.Location) != loc {
			continue
		}
		if info.Status == "waiting_for_resources" {
			missing := make(map[string]int)
			for k, v := range info.WaitingFor {
				if v.Have < v.Need {
					missing[k] = v.Need - v.Have
				}
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

	slices.SortFunc(printers, func(a, b *models.Device) int {
		return cmp.Compare(times[a.Code.Alias()], times[b.Code.Alias()])
	})
	data = [][]string{}
	for _, p := range printers {
		data = append(data, []string{
			p.Code.Alias(), string(p.Location), dt(times[p.Code.Alias()]),
		})
	}
	printTable([]string{"Autofactory", "Location", "ETA"}, data)

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
	bp := getBP(name)
	log("Print time, per copy: %s", bp.PrintTime.Duration())

	home, _ := cmd.Flags().GetString("home")
	printers, err := getHomeFactories(home)
	if err != nil {
		return err
	}

	// Check each printer for available print slots, and eta
	var slots int
	type rec struct {
		delay   time.Duration
		eta     time.Duration
		toQueue int
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

	copies, _ := cmd.Flags().GetInt("repeat")
	for copies > 0 {
		// Sort the printers by next available
		slices.SortFunc(printers, func(a, b *models.CodeAlias) int {
			return cmp.Compare(plan[a.Alias()].eta, plan[b.Alias()].eta)
		})
		var found bool
		for _, p := range printers {
			pl := plan[p.Alias()]
			if pl.avail == 0 {
				continue
			}
			// Add the next print
			pl.toQueue++
			pl.eta = pl.eta + bp.PrintTime.Duration()
			plan[p.Alias()] = pl
			found = true
			break
		}
		if found {
			copies--
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

	var data [][]string
	for _, p := range printers {
		pl := plan[p.Alias()]
		if pl.toQueue == 0 {
			continue
		}
		var delay string
		if pl.delay > 0 {
			delay = dt(pl.delay)
		}
		data = append(data, []string{
			p.Alias(), d(pl.toQueue), delay, dt(pl.eta),
		})
		for pl.toQueue > 0 {
			_, err = rest.DeviceCommand[models.CommandResp](p, "enqueue_print", cfg)
			if err != nil {
				return err
			}
			pl.toQueue--
		}
	}
	printTable([]string{"Factory", "Copies", "Delay", "ETA"}, data)
	return nil
}
