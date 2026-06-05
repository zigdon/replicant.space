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
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		loc, _ := cmd.Flags().GetString("location")
		rd, err := rest.ReplicantDevices(rID, loc)
		if err != nil {
			return fmt.Errorf("Error getting replicant %q devices: %v", rID, err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(rd)
		} else {
			fmt.Printf("Replicant: %s\n", rID)
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
			}, data)

		}
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(devicesCmd)
	devicesCmd.PersistentFlags().StringP("location", "l", "", "Filter results to a specific location code")
}
