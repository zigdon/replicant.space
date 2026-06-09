package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
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
			r, err := rest.Replicant(rID)
			if err != nil {
				return err
			}
			printReplicantDeviceList(r)
		}
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(devicesCmd)
	devicesCmd.PersistentFlags().StringP("location", "l", "", "Filter results to a specific location code")
}

func printReplicantDeviceList(r *models.Replicant) {
	devs, err := rest.ReplicantDevices(r.ReplicantCode.String(), "")
	if err != nil {
		log(err.Error())
		return
	}
	ra, err := db.Alias(r.ReplicantCode.String(), "replicant")
	if err == nil {
		fmt.Printf("Replicant: %s (%s/%s @ %s)\n",
			r.Name, ra, r.ReplicantCode, r.CurrentLocation)
	} else {
		fmt.Printf("Replicant: %s (%s @ %s)\n",
			r.Name, r.ReplicantCode, r.CurrentLocation)
	}
	var data [][]string
	for _, d := range devs {
		status := d.Status
		if strings.Contains(status, "repairing (") {
			target := status[strings.Index(status, "(")+1 : strings.Index(status, ")")]
			status = fmt.Sprintf("repairing (%s)", alias(target))
		}
		data = append(data, []string{
			d.Type,
			alias(d.Code.String()),
			alias(d.ControllerDeviceCode.String()),
			b(d.InControlRange),
			d.Location,
			f(d.OperationalCapacity),
			status,
			alias(d.StowedInDeviceCode.String()),
		})
	}
	slices.SortFunc(data, func(a, b []string) int {
		return cmp.Or(
			cmp.Compare(a[0], b[0]),
			cmp.Compare(a[1], b[1]),
		)
	})
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
