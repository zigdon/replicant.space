package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// devicesCmd represents the devices command
var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List devices owned by a replicant",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		args = append(args, "owner", rID.String())
		return deviceListCmd.RunE(cmd, args)
	},
}

func init() {
	replicantCmd.AddCommand(devicesCmd)
	devicesCmd.Flags().StringP("location", "l", "", "Filter results to a specific location code")
	devicesCmd.Flags().Bool("ignore_tags", false, "If set, ignore tag filters")
	devicesCmd.Flags().StringSliceP("filter_tags", "t", []string{"infrastructure"}, "Filter results with these tags")
}
