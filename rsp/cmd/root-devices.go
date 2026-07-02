package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
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
		devs, err := rest.Devices(filter)
		if err != nil {
			return err
		}
		for _, d := range devs {
			d.Alias()
		}

		var origin *models.Position
		if src, _ := cmd.Flags().GetString("distance"); src != "" {
			if o, err := getInfo(models.NewCodeAlias(src)); err == nil {
				origin = o.GetPosition()
				log("Distance from %s: %s", o.Code.Alias(), origin)
			}
		}

		var tags []string
		if ignore, _ := cmd.Flags().GetBool("ignore_tags"); !ignore {
			tags, _ = cmd.Flags().GetStringSlice("filter_tags")
		}
		devs, skipped := filterDevices(devs, tags)
		printDeviceList(devs, origin)
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

			wg.Go(func() {
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
						loc = d.Location
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
	deviceListCmd.Flags().StringSliceP("filter_tags", "t", []string{"infrastructure", "mine", "matrix"}, "Filter results with these tags")
	deviceListCmd.Flags().StringP("distance", "d", "", "Show distance from this object's star")

	rootCmd.AddCommand(networkCmd)
}

func filterDevices(devs []*models.Device, tags []string) ([]*models.Device, map[string]int) {
	var ret []*models.Device
	var skipTags = make(map[string]bool)
	for _, t := range tags {
		skipTags[t] = true
	}
	var skipped = make(map[string]int)

	for _, d := range devs {
		if skipTags["matrix"] && d.Type == "replicant_matrix" && d.Status == "stowed" {
			skipped["matrix"]++
			continue
		}
		if skipTags["mine"] && slices.ContainsFunc(d.Tags, func(s string) bool {
			loc := strings.ToLower(d.Location)
			if loc == "" {
				return false
			}
			return slices.Contains(d.Tags, fmt.Sprintf("mine-%s", loc)) ||
				strings.Contains(s, "-"+loc[:strings.Index(loc, "-")]+"-")
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
		ret = append(ret, d)
	}

	return ret, skipped
}

func printDeviceList(devs []*models.Device, reference *models.Position) {
	var data [][]string

	for _, d := range devs {
		loc := d.GetPosition()
		status := d.Status
		if strings.Contains(status, "repairing (") {
			target := status[strings.Index(status, "(")+1 : strings.Index(status, ")")]
			status = fmt.Sprintf("repairing (%s)", alias(target))
		}
		line := []string{
			d.Type,
			d.Code.Alias(),
			d.ControllerDeviceCode.Alias(),
			d.Location,
			p(d.OperationalCapacity),
			status,
			d.StowedInDeviceCode.Alias() + d.AttachedToDeviceCode.Alias(),
			list(d.Tags),
			d.OwnerReplicant.Alias(),
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
		"With",
		"Tags",
		"Replicant",
	}
	if reference != nil {
		headers = append(headers, "Distance")
	}
	printTable(headers, data)
}
