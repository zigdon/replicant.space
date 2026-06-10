package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage device tags",
}

var taggedCmd = &cobra.Command{
	Use:   "list",
	Short: "List the tags on a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		tag, _ := cmd.Flags().GetString("tag")
		res, err := rest.GetTagged(tag)
		if err != nil { return err }
		var details [][]string
		for _, d := range res.Devices {
			code := alias(d.Code.String())
			code = lines([]string{code, unalias(code)})
			var totalCargo float32
			var cargo []string
			for _, c := range d.Cargo {
				totalCargo += c.Quantity
				cargo = append(cargo, fmt.Sprintf("%.2f x %s", c.Quantity, c.ResourceType))
			}
			cargo = append([]string{fmt.Sprintf("%.2f/%d (%.0f%%)",
			    totalCargo, d.CargoCapacity, totalCargo/float32(d.CargoCapacity)*100)}, cargo...)
			details = append(details, []string{code, d.Type, d.Location,
				d.Status, alias(d.ReplicantCode.String()),
				lines(cargo),
			})
		}

		printTable(
			[]string{"Code", "Type", "Location", "Status", "Replicant", "Cargo"},
			details,
		)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(tagCmd)
	tagCmd.AddCommand(taggedCmd)
	taggedCmd.Flags().StringP("tag", "t", "", "Tag to list")
	taggedCmd.MarkFlagRequired("tag")
}
