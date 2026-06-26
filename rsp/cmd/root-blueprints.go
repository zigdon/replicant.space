package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// blueprintsCmd represents the blueprints command
var blueprintsCmd = &cobra.Command{
	Use:   "blueprints",
	Short: "List owne blueprints",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := rest.Blueprints()
		if err != nil {
			return fmt.Errorf("Failed to get blueprints: %v", err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(res)
			return nil
		}
		var blues [][]string
		for _, b := range res.Blueprints {
			if filter, _ := cmd.Flags().GetString("filter"); filter != "" {
				if !strings.Contains(b.DeviceType, filter) {
					continue
				}
			}
			if feature, _ := cmd.Flags().GetString("feature"); feature != "" {
				if !slices.Contains(b.Features, feature) {
					continue
				}
			}
			var resources []string
			for k, v := range b.Resources {
				if v == 0 {
					continue
				}
				resources = append(resources, fmt.Sprintf("%4d x %s", v, k))
			}
			blues = append(blues, []string{
				b.DeviceType,
				wrap(list(b.Features), 20),
				b.PrintTime.String(),
				strings.Join(resources, "\n"),
				lines([]string{
					fmt.Sprintf("Attach: %d", b.AttachCapacity),
					fmt.Sprintf("Cargo: %d", b.CargoCapacity),
					fmt.Sprintf("Stow: %d", b.StowCapacity),
				}),
				wrap(b.Description, 40),
			})
		}
		printTable(
			[]string{"Type", "Features", "Print Time", "Resources", "Stats", "Description"}, blues,
		)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(blueprintsCmd)
	blueprintsCmd.Flags().StringP("filter", "f", "", "Only display blueprints that match the substring")
	blueprintsCmd.Flags().StringP("feature", "t", "", "Only display blueprints include the specified feature")
}
