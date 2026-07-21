package cmd

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
	"golang.org/x/sync/semaphore"
)

var deviceListCmd = &cobra.Command{
	Use:   "devices",
	Short: "List all the devices",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Args are filter pairs
		filter := make(map[string]string)
		for i := 0; i < len(args)-1; i += 2 {
			a := args[i]
			if len(args) < i+2 {
				return fmt.Errorf("Filter argument must be pairs, got: %v", args)
			}
			v := args[i+1]
			switch a {
			case "location":
				filter["location"] = v
			case "type":
				filter["device_type"] = v
			case "owner":
				filter["replicant_code"] = db.Dealias(v)
			default:
				return fmt.Errorf("Unknown filter %s", a)
			}
		}
		if f, ok := filter["device_type"]; ok {
			// Accept prefixes for types
			if t := db.GetTypeForPrefix(f); t != "" {
				filter["device_type"] = t
			}
		}
		refresh := getBool(cmd, "refresh")
		devs, err := rest.CachedDevices(filter, !refresh)
		if err != nil {
			return err
		}
		for _, d := range devs {
			d.Alias()
		}

		var origin *models.Position
		if src := getString(cmd, "distance"); src != "" {
			if db.Dealias(src) != src {
				// It's a device
				if o, err := getInfo(models.NewCodeAlias(src)); err == nil {
					origin = o.GetPosition()
					log("Distance from %s: %s", o.Code.Alias(), origin)
				}
			} else {
				// It's a location (probably)
				oStar, err := models.NewStar(src)
				if err == nil {
					origin = oStar.Position
					log("Distance from %s: %s", src, origin)
				}
			}
		}

		var skipped map[string]int
		ignore := getBool(cmd, "ignore_tags")
		only := getStringSlice(cmd, "only_tags")
		filterTags := getStringSlice(cmd, "filter_tags")
		merge := getBool(cmd, "merge")
		if !ignore {
			devs, skipped = filterDevices(devs, filterTags, only)
		}
		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(devs)
			return nil
		}
		printDeviceList(devs, origin, merge)
		var stats []string
		for k, v := range skipped {
			if v == 0 {
				continue
			}
			stats = append(stats, fmt.Sprintf("%s: %d", k, v))
		}
		slices.Sort(stats)
		log(lines(stats))
		return nil
	},
}

var networkCmd = &cobra.Command{
	Use:   "networks",
	Short: "List all networks the FTL relays create",
	RunE: func(cmd *cobra.Command, args []string) error {
		devs, err := rest.Devices(nil)
		if err != nil {
			return err
		}
		networks := []*models.Network{}
		var inactive [][]string
		var wg sync.WaitGroup
		var errs []error
		var mu sync.Mutex
		sem := semaphore.NewWeighted(10)
		ctx := context.Background()
		for _, d := range devs {
			if d.Type != "ftl_relay" {
				continue
			}
			var found bool
			for _, n := range networks {
				if slices.Contains(n.Devices(), d.Code.String()) {
					found = true
					break
				}
			}
			if found {
				continue
			}

			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}

			wg.Go(func() {
				defer sem.Release(1)
				net, err := rest.DeviceNetwork(d.Code)
				if err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
					return
				}
				if net == nil || net.Status != "relaying" {
					loc := d.StowedInDeviceCode.Alias()
					if loc == "" {
						loc = string(d.Location)
					}
					mu.Lock()
					inactive = append(inactive, []string{d.Code.Alias(), loc})
					mu.Unlock()
					return
				}
				mu.Lock()
				defer mu.Unlock()
				if slices.ContainsFunc(networks, func(n *models.Network) bool {
					return n.Equal(net)
				}) {
					return
				}
				networks = append(networks, net)
			})
		}
		wg.Wait()
		slices.SortFunc(inactive, func(a, b []string) int {
			return cmp.Compare(a[1], b[1])
		})
		slices.SortFunc(networks, func(a, b *models.Network) int {
			if len(a.Connections) == 0 {
				return -1
			}
			if len(b.Connections) == 0 {
				return 1
			}
			return cmp.Compare(a.Connections[0].Star, b.Connections[0].Star)
		})
		var data [][]string
		for i, n := range networks {
			var aliases []string
			for _, d := range n.Devices() {
				aliases = append(aliases, fmt.Sprintf("%s (%s)", alias(d), d))
			}
			data = append(data, []string{d(i), lines(n.Stars()), lines(aliases)})
		}
		printTable([]string{"ID", "Stars", "Devices"}, data)
		printTable([]string{"Inactive Relays", "Location"}, inactive)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deviceListCmd)
	deviceListCmd.Flags().Bool("ignore_tags", false, "If set, ignore tag filters")
	deviceListCmd.Flags().Bool("refresh", false, "If set, bypass the cached data")
	deviceListCmd.Flags().StringSliceP("filter_tags", "f", []string{"infrastructure", "mine", "matrix"}, "Filter results with these tags")
	deviceListCmd.Flags().StringSliceP("only_tags", "t", []string{}, "Show only results with these tags")
	deviceListCmd.Flags().StringP("distance", "d", "", "Show distance from this object's star")
	deviceListCmd.Flags().Bool("merge", true, "If set, group duplicate devices")

	rootCmd.AddCommand(networkCmd)
}

func filterDevices(devs []*models.Device, withoutTags, withTags []string) ([]*models.Device, map[string]int) {
	var ret []*models.Device
	var skipTags = make(map[string]bool)
	var onlyTags = make(map[string]bool)
	for _, t := range withoutTags {
		skipTags[t] = true
	}
	for _, t := range withTags {
		onlyTags[t] = true
	}
	var skipped = make(map[string]int)

	for _, d := range devs {
		if len(withTags) > 0 {
			if len(d.Tags) == 0 {
				continue
			}
			var keep bool
			for _, t := range d.Tags {
				if onlyTags[t] {
					keep = true
					break
				}
			}
			if !keep {
				continue
			}
		} else {
			if skipTags["matrix"] && strings.Contains(d.Type, "matrix") && (d.Status == "stowed" || d.Status == "idle") {
				skipped["matrix"]++
				continue
			}
			if skipTags["mine"] && slices.ContainsFunc(d.Tags, func(s string) bool {
				loc := strings.ToLower(string(d.Location))
				if loc == "" {
					return false
				}
				if s == fmt.Sprintf("mine-%s", loc) {
					return true
				}
				idx := strings.Index(loc, "-")
				return idx > 0 && strings.Contains(s, "-"+loc[:idx]+"-")
			}) {
				skipped["mines"]++
				continue
			}
			skip := false
			for _, tag := range d.Tags {
				if s := skipTags[tag]; s {
					skipped[fmt.Sprintf("%s: %s", d.Type, tag)]++
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}
		ret = append(ret, d)
	}

	return ret, skipped
}

func printDeviceList(devs []*models.Device, reference *models.Position, merge bool) {
	var data [][]string
	mkKey := func(dev *models.Device, eta string) string {
		return strings.Join([]string{
			dev.Type,
			dev.ControllerDeviceCode.Alias(),
			string(dev.Location),
			dev.Status,
			eta,
			dev.StowedInDeviceCode.Alias() + dev.AttachedToDeviceCode.Alias(),
			list(dev.Tags),
			d(len(dev.AttachedDevices)),
		}, "|")
	}

	dups := make(map[string][]*models.Device)
	for _, d := range devs {
		loc := d.GetPosition()
		status := d.Status
		var eta string
		if strings.Contains(status, "repairing (") {
			target := status[strings.Index(status, "(")+1 : strings.Index(status, ")")]
			status = fmt.Sprintf("repairing (%s)", alias(target))
			eta = fmt.Sprintf("%.0f%% %s", d.Repair.ProgressPercent, d.Repair.Eta.String())
		} else if d.Travel != nil {
			eta = fmt.Sprintf("%.0f%% %s", d.Travel.ProgressPercent, d.Travel.Eta.String())
		} else if d.Prospect != nil {
			eta = fmt.Sprintf("%.0f%% %s",
				d.Prospect.ProgressPercent, time.Until(d.Prospect.Completes.Time()).Truncate(time.Second))
		} else if d.Compact != nil {
			eta = fmt.Sprintf("%.0f%% %s",
				d.Compact.ProgressPercent, time.Until(d.Compact.Completes.Time()).Truncate(time.Second))
		} else if d.Unfurl != nil {
			eta = fmt.Sprintf("%.0f%% %s",
				d.Unfurl.ProgressPercent, time.Until(d.Unfurl.Completes.Time()).Truncate(time.Second))
		} else if d.Scan != nil {
			eta = fmt.Sprintf("%.0f%% %s",
				d.Scan.ProgressPercent, d.Scan.Eta.Duration().Truncate(time.Second))
		} else if d.Printing != nil {
			eta = fmt.Sprintf("%.0f%% %s",
				d.Printing.ProgressPercent, d.Printing.Eta.Duration().Truncate(time.Second))
		}
		key := mkKey(d, eta)
		if _, ok := dups[key]; merge && ok {
			dups[key] = append(dups[key], d)
			continue
		} else {
			dups[key] = []*models.Device{d}
		}
		var attached string
		if d.AttachCapacity > 0 {
			attached = fmt.Sprintf("%d/%d", len(d.AttachedDevices), d.AttachCapacity)
		}
		line := []string{
			d.Type,
			d.Code.Alias(),
			d.ControllerDeviceCode.Alias(),
			string(d.Location),
			p(d.OperationalCapacity),
			status,
			eta,
			d.StowedInDeviceCode.Alias() + d.AttachedToDeviceCode.Alias(),
			list(d.Tags),
			d.OwnerReplicant.Alias(),
			attached,
			key,
		}
		if reference != nil {
			if loc != nil {
				line = append(line, f(loc.Distance(reference)))
			} else {
				line = append(line, "")
			}
		}
		data = append(data, line)
	}
	for _, l := range data {
		dp := len(dups[l[11]])
		if dp > 1 {
			l[11] = d(dp)
		} else {
			l[11] = ""
		}
	}
	slices.SortFunc(data, func(a, b []string) int {
		return cmp.Or(
			cmp.Compare(a[0], b[0]),
			cmp.Compare(a[1], b[1]),
		)
	})
	headers := []string{
		"Type",
		"Code",
		"Controller",
		"Location",
		"Ops %",
		"Status",
		"ETA",
		"With",
		"Tags",
		"Replicant",
		"Attached",
		"Duplicates",
	}
	if reference != nil {
		headers = append(headers, "Distance")
	}
	printTable(headers, data)
}
