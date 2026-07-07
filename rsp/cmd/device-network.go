package cmd

import (
	"cmp"
	"slices"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var deviceNetworkCmd = &cobra.Command{
	Use:   "network",
	Short: "Show devices networked together",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		res, err := rest.DeviceNetwork(models.NewCodeAlias(id))
		if err != nil {
			return err
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(res)
			return nil
		}
		printTable(
			[]string{"Status", "Range LY"},
			[][]string{{res.Status, f(res.RangeLy)}},
		)
		var nodes [][]string
		for _, n := range res.Connections {
			s := &models.Star{Designation: n.Star}
			if err := s.Get(); err != nil {
				log("Unknown star %q", n.Star)
			}
			nodes = append(nodes, []string{
				n.Star, n.DeviceCode.Alias(), f(n.DistanceLy),
				s.Position.String(),
			})
		}
		slices.SortFunc(nodes, func(a, b []string) int {
			return cmp.Compare(a[0], b[0])
		})
		printTable([]string{"Star", "Device", "Distance LY", "Position"}, nodes)
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceNetworkCmd)
}
