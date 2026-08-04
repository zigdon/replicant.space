package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
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
	rootPrintCmd.Flags().BoolP("dry_run", "n", false, "Don't actually print, only plan")
	rootPrintCmd.Flags().Bool("use_inventory", true, "Skip printing existing components")
	rootPrintCmd.Flags().BoolP("flatpack", "f", false, "If set, print packed for travel")
	rootPrintCmd.Flags().BoolP("unfurl", "u", false, "If set, print unpacked")
	rootPrintCmd.Flags().StringSliceP("tag", "t", []string{}, "Tags to add to the printed device")

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
		if slices.Contains([]string{"compacted", "compacting", "unfurling"}, f.Status) {
			log("Skipping %s: %s", f.Code.Alias(), f.Status)
			continue
		}
		printers = append(printers, f.Code)
	}
	return printers, nil
}

func rootPrintList(cmd *cobra.Command, args []string) error {
	loc := getString(cmd, "location")
	refresh := getBool(cmd, "refresh")
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
			if len(info.PrintQueue) > 0 {
				pending := info.PrintQueue[0]
				queue = append(queue, pq{
					location:   info.Location,
					code:       info.Code,
					deviceType: pending.Type,
					tags:       append([]string{"Waiting for resources"}, pending.Tags...),
					pos:        -1,
					missing:    missing,
				})
			} else {
				queue = append(queue, pq{
					location: info.Location,
					code:     info.Code,
					tags:     append([]string{"Waiting for resources"}, info.Tags...),
					pos:      -1,
					missing:  missing,
				})
			}
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
	var data [][]any
	for _, q := range queue {
		var pos any
		if q.pos < 0 {
			pos = "printing"
		} else {
			pos = q.pos
		}
		data = append(data, []any{
			q.location, q.deviceType, list(q.tags), q.code.Alias(), pos, q.eta, rm(q.missing),
		})
	}
	printTable([]string{"Location", "Type", "Tags", "Factory", "Position", "ETA", "Missing"}, data)

	if len(totalMissing) > 0 {
		data = [][]any{}
		for k, v := range totalMissing {
			data = append(data, []any{k, v})
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

	fp := getBool(cmd, "flatpack")
	uf := getBool(cmd, "unfurl")
	if fp && uf {
		return fmt.Errorf("Only one of --flatpack or --unfurl can be specified")
	}
	bp := common.GetBP(name)
	if slices.Contains(bp.Features, "modular") {
		if !fp && !uf {
			return fmt.Errorf("Modular blueprints must specified either --flatpack or --unfurl")
		}
	} else {
		if fp || uf {
			return fmt.Errorf("%s is not modular, --flatpack and --unfurl are invalid", name)
		}
	}

	home := getString(cmd, "home")
	copies := getInt(cmd, "repeat")
	controller := getString(cmd, "controller")
	onComplete := getString(cmd, "on_complete")
	tags := getStringSlice(cmd, "tag")
	cfg := make(map[string]any)
	if fp {
		cfg["flatpack"] = true
	} else if uf {
		cfg["flatpack"] = false
	}
	if controller != "" {
		cfg["controller"] = controller
	}
	if onComplete != "" {
		cfg["oncomplete"] = onComplete
	}
	if len(tags) > 0 {
		cfg["tags"] = tags
	}

	_, err := common.Print(home, name, copies, true, getBool(cmd, "dry_run"), cfg)

	return err
}
