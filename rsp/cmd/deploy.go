package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a device",
	Run: func(cmd *cobra.Command, args []string) {
		id, err := cmd.Flags().GetString("device")
		if err != nil {
			log("Error: %v", err)
			return
		}
		if id == "" {
			log("Device ID is required, pass --device or -d")
			return
		}
		resp, err := rest.DeviceCommand(id, "deploy")
		if err != nil {
			log("%q deploy failed: %v", id, err)
			return
		}
		prettyPrint(resp)
	},
}

func init() {
	deviceCmd.AddCommand(deployCmd)
}
