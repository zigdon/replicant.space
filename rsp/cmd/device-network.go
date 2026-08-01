package cmd

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var deviceNetworkCmd = &cobra.Command{
	Use:   "network",
	Short: "Show devices networked together",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := getString(cmd, "device")
		ca := models.NewCodeAlias(id)
		res, err := rest.DeviceNetwork(ca)
		if err != nil {
			return err
		}
		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(res)
			return nil
		}
		printTable(
			[]string{"Status", "Range LY"},
			[][]any{{res.Status, res.RangeLy}},
		)
		ref := getString(cmd, "reference")
		var star *models.Star
		if ref == "" {
			di, err := getInfo(ca)
			if err != nil {
				return fmt.Errorf("Can't get info for %q: %v", ca.Alias(), err)
			}
			starName := di.Location.Star()
			star, err = models.NewStar(starName)
		} else {
			star, err = models.NewStar(ref)
		}
		if err != nil {
			return fmt.Errorf("Can't get cached star %q: %v", star.Designation, err)
		}
		for _, n := range res.Connections {
			s, err := models.NewStar(n.Star)
			if err != nil {
				return err
			}
			n.DistanceLy = s.Position.Distance(star.Position)
		}
		if ref != "" {
			slices.SortFunc(res.Connections, func(a, b *models.NetworkNode) int {
				return cmp.Compare(b.DistanceLy, a.DistanceLy)
			})
		}

		var nodes [][]any
		for _, n := range res.Connections {
			s, _ := models.NewStar(n.Star)
			nodes = append(nodes, []any{
				n.Star, n, s.Position.Distance(star.Position), s.Position.String(),
			})
		}
		if ref == "" {
			slices.SortFunc(nodes, func(a, b []any) int {
				return cmp.Compare(a[0].(string), b[0].(string))
			})
		}
		printTable([]string{"Star", "Device", "Distance LY", "Position"}, nodes)
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceNetworkCmd)
	deviceNetworkCmd.Flags().StringP("reference", "r", "", "If set, show distances from this reference rather than the device")
}
