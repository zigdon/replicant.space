package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// locationCmd represents the location command
var locationCmd = &cobra.Command{
	Use:   "location",
	Short: "List the contents of a location",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || args[0] == "" {
			return fmt.Errorf("Location is required, pass as an argument")
		}
		res, err := rest.Location(args[0])
		if err != nil {
			return fmt.Errorf("Failed to get location %q: %v", args[0], err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw { prettyPrint(res) }

		var data [][]string
		for _, i := range res.Inventory {
			data = append(data, []string{i.ResourceType, f(i.Quantity)})
		}
		printTable([]string{"Resource", "Quantity"}, data)

		data = [][]string{}
		for _, d := range res.Devices {
			data = append(data, []string{d.Code, d.Type, d.Status})
		}
		printTable([]string{"Device Code", "Type", "Status"}, data)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(locationCmd)
}
