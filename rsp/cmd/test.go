package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := make(map[string]string)
		for i := 0; i < len(args); i += 2 {
			cfg[args[i]] = args[i+1]
		}
		devs, err := rest.Devices(cfg)
		if err != nil {
			return err
		}
		var data [][]string
		for _, d := range devs {
			data = append(data, []string{
				d.Code.Alias(), d.Location, d.Type, d.Status,
				d.AttachedToDeviceCode.Alias(), d.StowedInDeviceCode.Alias(),
				list(d.Tags),
			})
		}
		printTable([]string{
			"Alias", "Location", "Type", "Status", "Attached to", "Stowed in", "Tags",
		}, data)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
