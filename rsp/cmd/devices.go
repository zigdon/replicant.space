package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// devicesCmd represents the devices command
var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List devices owned by a replicant",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, _ := cmd.Flags().GetString("code")
		if rID == "" {
			id, _ := cmd.Flags().GetInt("id")
			code, err := rest.ReplicantID(id)
			if err != nil {
				return fmt.Errorf("Replicant #%d not found: %v", id, err)
			}
			rID = code
		}
		rd, err := rest.ReplicantDevices(rID)
		if err != nil {
			return fmt.Errorf("Error getting replicant %q devices: %v", rID, err)
		}
		prettyPrint(rd)
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(devicesCmd)
}
