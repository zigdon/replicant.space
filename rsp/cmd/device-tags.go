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

var addTagCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a tag to a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		res, err := rest.UpdateTags(id, rest.AddTag, args)
		if err != nil {
			return err
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(res)
			return nil
		}
		printTable([]string{"Device", "Tags"}, [][]string{{
			res.Code.Alias(), lines(res.Tags),
		}})
		return nil
	},
}

var delTagCmd = &cobra.Command{
	Use:   "del",
	Short: "Remove a tag from a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		res, err := rest.UpdateTags(id, rest.DelTag, args)
		if err != nil {
			return err
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(res)
			return nil
		}
		printTable([]string{"Device", "Tags"}, [][]string{{
			res.Code.Alias(), lines(res.Tags),
		}})
		return nil
	},
}

var findTagsCmd = &cobra.Command{
	Use:   "find",
	Short: "Find devices with a given tag",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("A tag must be specified")
		}
		res, err := rest.GetTagged(args[0])
		if err != nil {
			return err
		}
		var details [][]string
		for _, d := range res.Devices {
			code := d.Code.Alias()
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
				d.Status, d.ReplicantCode.Alias(),
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
	rootCmd.AddCommand(findTagsCmd)

	deviceCmd.AddCommand(tagCmd)
	tagCmd.AddCommand(addTagCmd)
	tagCmd.AddCommand(delTagCmd)
}
