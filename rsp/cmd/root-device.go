package cmd

import (
	"github.com/spf13/cobra"
)

// deviceCmd represents the device command
var deviceCmd = &cobra.Command{
	Use:   "device",
	Short: "Manage devices",
	RunE:  infoCmd.RunE,
}

func init() {
	rootCmd.AddCommand(deviceCmd)
	deviceCmd.PersistentFlags().StringP("device", "d", "", "Device to use (e.g. A1B2C3D4 or md-1)")
	deviceCmd.MarkPersistentFlagRequired("device")
}
