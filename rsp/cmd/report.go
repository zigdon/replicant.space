package cmd

import "github.com/spf13/cobra"

var reportCmd = &cobra.Command{
	Short: "report",
	Use: "Generate reports entirely from the cache",
}

var travelReportCmd = &cobra.Command{
	Short: "travel",
	Use: "Display information about devices in motion",
	RunE: func(cmd *cobra.Command, args []string) error {
		// select array_agg(code), dest from (
		//     select code, split_part(data->'travel'->>'destination', '-', 1) dest
		//     from json_devices
		//     where data->'travel'->'destination' is not null
		// ) group by dest

		return nil
	},
}
