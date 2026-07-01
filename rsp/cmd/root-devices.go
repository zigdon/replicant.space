package cmd

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var deviceListCmd = &cobra.Command{
	Use:   "devices",
	Short: "List all the devices on the account",
	RunE: func(cmd *cobra.Command, args []string) error {
		acc, err := rest.Account()
		if err != nil {
			return err
		}
		var names []string
		for name := range acc.Replicants {
			names = append(names, name)
		}
		slices.Sort(names)

		var filter []string
		if ignore, _ := cmd.Flags().GetBool("ignore_tags"); !ignore {
			filter, _ = cmd.Flags().GetStringSlice("filter_tags")
		}
		owner, _ := cmd.Flags().GetString("replicant")
		var oca *models.CodeAlias
		if owner != "" {
			oca = models.NewCodeAlias(owner)
		}
		location, _ := cmd.Flags().GetString("location")
		for _, n := range names {
			printReplicantDeviceList(acc.Replicants[n], filter, oca, location)
			if acc.ReplicantCooperation == "shared" {
				break
			}
		}
		return nil
	},
}

var networkCmd = &cobra.Command{
	Use:   "networks",
	Short: "List all networks the FTL relays create",
	RunE: func(cmd *cobra.Command, args []string) error {
		devs, err := rest.AllDevices()
		if err != nil {
			return err
		}
		networks := []*models.Network{}
		var inactive [][]string
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

			net, err := rest.DeviceNetwork(d.Code)
			if err != nil {
				return err
			}
			if net == nil || net.Status != "relaying" {
				loc := d.StowedInDeviceCode.Alias()
				if loc == "" {
					loc = d.Location
				}
				inactive = append(inactive, []string{d.Code.Alias(), loc})
				continue
			}
			for _, n := range networks {
				if n.Equal(net) {
					found = true
					break
				}
			}
			if found {
				continue
			}
			networks = append(networks, net)
		}
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
	deviceListCmd.Flags().StringP("replicant", "r", "", "Show devices owned by this replicant, default all")
	deviceListCmd.Flags().StringP("location", "l", "", "Only show devices in this location")

	rootCmd.AddCommand(networkCmd)
}
