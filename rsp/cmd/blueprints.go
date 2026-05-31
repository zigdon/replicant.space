package cmd

import (
	"fmt"
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
		} else {
			var blues [][]string
			for _, b := range res.Blueprints {
				if filter, _ := cmd.Flags().GetString("filter"); filter != "" {
					if !strings.Contains(b.DeviceType, filter) {
						continue
					}
				}
				var resources []string
				for k, v := range b.Resources {
					if v == 0 { continue }
					resources = append(resources, fmt.Sprintf("%4d x %s", v, k))
				}
				blues = append(blues, []string{
					b.DeviceType,
					list(b.Features),
					f(b.PrintTime),
					strings.Join(resources, "\n"),
				})
			}
			printTable(
				[]string{},
				blues,
				0,
			)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(blueprintsCmd)
	blueprintsCmd.Flags().StringP("filter", "f", "", "Only display blueprints that match the substring")
}
