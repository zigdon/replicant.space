package cmd

import (
	"cmp"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var deviceNetworkCmd = &cobra.Command{
	Use:   "network",
	Short: "Show devices networked together",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		ca := models.NewCodeAlias(id)
		res, err := rest.DeviceNetwork(ca)
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
		ref, _ := cmd.Flags().GetString("reference")
		var star *models.Star
		if ref == "" {
			di, err := getInfo(ca)
			if err != nil {
				return err
			}
			starName, _, _ := strings.Cut(di.Location, "-")
			star = &models.Star{Designation: starName}
		} else {
			star = &models.Star{Designation: ref}
		}
		if err := star.Get(); err != nil {
			return err
		}
		var nodes [][]string
		for _, n := range res.Connections {
			s := &models.Star{Designation: n.Star}
			if err := s.Get(); err != nil {
				log("Unknown star %q", n.Star)
			}
			nodes = append(nodes, []string{
				n.Star, n.DeviceCode.Alias(), f(s.Position.Distance(star.Position)),
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
	deviceNetworkCmd.Flags().StringP("reference", "r", "", "If set, show distances from this reference rather than the device")
}
