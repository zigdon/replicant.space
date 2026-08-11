package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

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

var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Look up other devices in the system",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		dt := getString(cmd, "type")
		if a := db.GetTypeForPrefix(dt); a != "" {
			dt = a
		}
		res, err := rest.ReplicantPing(rID, getString(cmd, "owner"), dt)
		if err != nil {
			return err
		}
		if getBool(cmd, "raw") {
			prettyPrint(res)
			return nil
		}

		printTable([]string{"Star", "Device count"}, [][]any{{res.Star, res.DeviceCount}})

		var data [][]any
		for _, d := range res.Devices {
			data = append(data, []any{
				d.DeviceCode, d.DeviceType, d.Location, d.OwnerName, d.OwnerReplicantCode,
			})
		}
		printTable([]string{"Code", "Type", "Location", "Owner Name", "Owner Code"}, data)

		return nil
	},
}

func init() {
	replicantCmd.AddCommand(devicesCmd)
	devicesCmd.Flags().StringP("location", "l", "", "Filter results to a specific location code")
	devicesCmd.Flags().Bool("ignore_tags", false, "If set, ignore tag filters")
	devicesCmd.Flags().StringSliceP("filter_tags", "t", []string{"infrastructure"}, "Filter results with these tags")

	replicantCmd.AddCommand(pingCmd)
	pingCmd.Flags().StringP("owner", "o", "", "Only ping devices owned by this ID")
	pingCmd.Flags().StringP("type", "t", "", "Only ping devices of this type")
}
