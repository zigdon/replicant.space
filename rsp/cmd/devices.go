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
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(rd)
		} else {
			var data [][]string
			for _, d := range rd {
				data = append(data, []string{
					d.Type,
					d.Code,
					d.ControllerDeviceCode,
					b(d.InControlRange),
					d.Location,
					f(d.OperationalCapacity),
					d.Status,
					d.StowedInDeviceCode,
				})
			}
			printTable([]string{
				"Type",
				"Code",
				"Controller",
				"In range",
				"Location",
				"Operational Capacity",
				"Status",
				"Stowed in",
			}, data, 0)

		}
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(devicesCmd)
}
