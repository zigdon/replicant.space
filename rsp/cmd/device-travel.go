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
		via, _ := cmd.Flags().GetStringSlice("via")
		resp, err := rest.DeviceCommand[models.CommandResp](models.NewCodeAlias(id), "travel", map[string]any{
			"destination": args[0],
			"via":         via,
		})
		if err != nil {
			return fmt.Errorf("Failed to initiate travel for %q: %v", id, err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(resp)
		} else {
			printTable([]string{
				"Status", "ETA",
			}, [][]string{{
				resp.Status, resp.Eta.String(),
			}})
		}
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceTravelCmd)
	deviceTravelCmd.Flags().StringSliceP("via", "v", []string{}, "Specify an explicit route")
}
