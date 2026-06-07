package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var deliveryCmd = &cobra.Command{
	Use: "deliver",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		route, _ := cmd.Flags().GetString("srcdst")
		if !strings.Contains(route, ":") {
			return fmt.Errorf("--route must be of the form 'from:to'")
		}
		bits := strings.Split(route, ":")

		rs, _ := cmd.Flags().GetStringSlice("resources")
		resMap := make(map[string]int)
		for _, r := range rs {
			if !strings.Contains(r, ":") {
				return fmt.Errorf("--resources %q is not of the form type:qty", r)
			}
			rBits := strings.Split(r, ":")
			qty, err := strconv.Atoi(rBits[1])
			if err != nil {
				return fmt.Errorf("Can't parse %q: %v", rBits[1], err)
			}
			resMap[rBits[0]] += qty
		}

		cfg := map[string]any{
			"directive": "delivery",
			"configuration": map[string]any{
				"route": map[string]string{
					"collect": bits[0],
					"deliver": bits[1],
				},
				"requirement": resMap,
			},
		}

		res, err := rest.DeviceCommand(id, "set_directive", cfg)
		if err != nil {
			return fmt.Errorf("Can't set directive: %v", err)
		}
		prettyPrint(res)
		return nil
	},
}

func init() {
	mkDeviceCommand(
		"assemble", "Bring the fleet home to the controller's current location without ending the directive", "assemble", nil,
	)
	mkDeviceCommand(
		"clear_directive", "Drop the current directive entirely", "clear_directive", nil,
	)
	dirCmd := mkDeviceCommand(
		"directive", "Update the automation policy for a device", "set_directive",
		[]flagDesc{
			{
				name: "new_directive", short: 'n', required: true, jsonKey: "directive",
			},
			{
				name: "configuration", short: 'c', required: false,
				jsonKey: "configuration", mapFlag: true,
			}},
	)
	mkDeviceCommand(
		"launch", "Deploy the fleet and start executing the current directive", "launch", nil,
	)
	mkDeviceCommand(
		"withdraw", "Recall the fleet and pause execution", "withdraw", nil,
	)

	dirCmd.AddCommand(deliveryCmd)
	deliveryCmd.Flags().StringP("srcdst", "s", "", "source:dest location codes")
	deliveryCmd.Flags().StringSliceP("resources", "r", []string{}, "resources to collect, type:qty, repeatable")
	deliveryCmd.MarkFlagRequired("srcdst")
	deliveryCmd.MarkFlagRequired("resources")

}
