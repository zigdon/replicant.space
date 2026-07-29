package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
)

var deviceTravelCmd = &cobra.Command{
	Use:   "travel",
	Short: "Instruct a device to relocate",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || args[0] == "" {
			return fmt.Errorf("Destination is required for travel, pass as arg")
		}
		dest := args[0]
		id := models.NewCodeAlias(getString(cmd, "device"))
		dryRun := getBool(cmd, "dry_run")
		via := getStringSlice(cmd, "via")
		eta, err := common.Travel(id, dest, dryRun, via...)
		if err != nil {
			return fmt.Errorf("Failed to initiate travel for %q: %v", id, err)
		}
		log("In transit, ETA: %s (%s)", eta, time.Until(eta))

		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceTravelCmd)
	deviceTravelCmd.Flags().BoolP("dry_run", "n", false, "Only plot the route")
	deviceTravelCmd.Flags().StringSliceP("via", "v", []string{}, "Specify an explicit route")
}
