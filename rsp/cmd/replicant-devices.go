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
			return nil
		}
		r, err := rest.Replicant(rID)
		if err != nil {
			return err
		}
		var filter []string
		if ignore, _ := cmd.Flags().GetBool("ignore_tags"); !ignore {
			filter, _ = cmd.Flags().GetStringSlice("filter_tags")
		}
		printReplicantDeviceList(r, filter, rID, loc)
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(devicesCmd)
	devicesCmd.Flags().StringP("location", "l", "", "Filter results to a specific location code")
	devicesCmd.Flags().Bool("ignore_tags", false, "If set, ignore tag filters")
	devicesCmd.Flags().StringSliceP("filter_tags", "t", []string{"infrastructure"}, "Filter results with these tags")
}

func printReplicantDeviceList(r *models.Replicant, filterTags []string, owner *models.CodeAlias, location string) {
	devs, err := rest.ReplicantDevices(r.Code, "")
	if err != nil {
		log(err.Error())
		return
	}
	ra, err := db.Alias(r.Code.String(), "replicant")
	if err == nil {
		fmt.Printf("Replicant: %s (%s/%s @ %s)\n",
			r.Name, ra, r.Code, r.CurrentLocation)
	} else {
		fmt.Printf("Replicant: %s (%s @ %s)\n",
			r.Name, r.Code, r.CurrentLocation)
	}

	var data [][]string
	var skippedCnt int
	skipped := make(map[string]int)
	skipTags := make(map[string]bool)
	for _, tag := range filterTags {
		skipTags[tag] = true
	}

	for _, d := range devs {
		if owner != nil && d.OwnerReplicant != owner {
			continue
		}
		if location != "" && !strings.Contains(strings.ToLower(d.Location), strings.ToLower(location)) {
			continue
		}
		if skipTags["mine"] && slices.Contains(d.Tags, fmt.Sprintf("mine-%s", strings.ToLower(d.Location))) {
			skipped[fmt.Sprintf("mine-%s", strings.ToLower(d.Location))]++
			skippedCnt++
			continue
		}
		skip := false
		for _, tag := range d.Tags {
			if s := skipTags[tag]; s {
				skipped[fmt.Sprintf("%s: %s", d.Type, tag)]++
				skip = true
				break
			}
		}
		if skip {
			skippedCnt++
			continue
		}
		status := d.Status
		if strings.Contains(status, "repairing (") {
			target := status[strings.Index(status, "(")+1 : strings.Index(status, ")")]
			status = fmt.Sprintf("repairing (%s)", alias(target))
		}
		data = append(data, []string{
			d.Type,
			d.Code.Alias(),
			d.ControllerDeviceCode.Alias(),
			b(d.InControlRange),
			d.Location,
			f(d.OperationalCapacity),
			status,
			d.StowedInDeviceCode.Alias(),
			list(d.Tags),
			d.OwnerReplicant.Alias(),
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
		"Tags",
		"Replicant",
	}, data)
	if len(skipped) > 0 {
		fmt.Printf("Skipped %d devices:\n", skippedCnt)
		for k, v := range skipped {
			fmt.Printf("  %s: %d\n", k, v)
		}
	}
}
