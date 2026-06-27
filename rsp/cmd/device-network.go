package cmd

import (
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
			nodes = append(nodes, []string{
				n.Star, n.DeviceCode.Alias(), f(n.DistanceLy),
			})
		}
		printTable([]string{"Star", "Device", "Distance LY"}, nodes)
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceNetworkCmd)
}
