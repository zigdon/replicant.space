package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var deviceTravelCmd = &cobra.Command{
	Use:   "travel",
	Short: "Instruct a device to relocate",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || args[0] == "" {
			return fmt.Errorf("Destination is required for travel, pass as arg")
		}
		id, _ := cmd.Flags().GetString("device")
		cfg := map[string]any{
			"destination": args[0],
		}
		if dryRun, _ := cmd.Flags().GetBool("dry_run"); dryRun {
			cfg["dry_run"] = true
		}
		if via, _ := cmd.Flags().GetStringSlice("via"); len(via) > 0 {
			cfg["via"] = via
		}
		resp, err := rest.DeviceCommand[models.CommandResp](models.NewCodeAlias(id), "travel", cfg)
		if err != nil {
			return fmt.Errorf("Failed to initiate travel for %q: %v", id, err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(resp)
			return nil

		}

		var origin, dest []string
		origin = append(origin, resp.Origin)
		origin = append(origin, t(resp.Departed.Time()))
		dest = append(dest, resp.Destination)
		dest = append(dest, t(resp.Arrives.Time()))
		printTable([]string{"Status", "Departed", "Destination", "Total Time"},
			[][]string{{resp.Status, lines(origin), lines(dest), resp.TotalTime.String()}})
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceTravelCmd)
	deviceTravelCmd.Flags().BoolP("dry_run", "n", false, "Only plot the route")
	deviceTravelCmd.Flags().StringSliceP("via", "v", []string{}, "Specify an explicit route")
}
