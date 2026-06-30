package cmd

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var deliveryCmd = &cobra.Command{
	Use: "deliver",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		route, _ := cmd.Flags().GetString("route")
		if !strings.Contains(route, ":") {
			return fmt.Errorf("--route must be of the form 'from:to', got %q", route)
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

		res, err := rest.DeviceCommand(models.NewCodeAlias(id), "set_directive", cfg)
		if err != nil {
			return fmt.Errorf("Can't set directive: %v", err)
		}

		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(res)
			return nil
		}
		printTable([]string{
			"Directive", "Status", "Configure",
		}, [][]string{{
			res.AmiDirective.Name, res.AmiDirectiveStatus, m(res.AmiDirective.Config),
		}})
		return nil
	},
}

var surveyCmd = &cobra.Command{
	Use: "survey_system",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")

		cfgPlanets := "all"
		if noPlanets, _ := cmd.Flags().GetBool("no_planets"); noPlanets {
			cfgPlanets = "none"
		}
		cfgMoons := "all"
		if noMoons, _ := cmd.Flags().GetBool("no_moons"); noMoons {
			cfgMoons = "none"
		}
		noRecall, _ := cmd.Flags().GetBool("no_recall")
		cfg := map[string]any{
			"directive": "survey_system",
			"configuration": map[string]any{
				"planets": cfgPlanets,
				"moons":   cfgMoons,
				"recall":  !noRecall,
			},
		}

		res, err := rest.DeviceCommand(models.NewCodeAlias(id), "set_directive", cfg)
		if err != nil {
			return fmt.Errorf("Can't set directive: %v", err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(res)
			return nil
		}
		printTable([]string{
			"Directive", "Planets", "Moons", "Recall", "Status",
		}, [][]string{{
			res.AmiDirective.Name, res.AmiDirective.Config["planets"].(string),
			res.AmiDirective.Config["moons"].(string),
			b(res.AmiDirective.Config["recall"].(bool)),
			res.AmiDirectiveStatus,
		}})
		return nil
	},
}

func init() {
	mkDeviceCommand(
		"assemble", "Bring the fleet home to the controller's current location without ending the directive", "assemble", nil, "",
	)
	mkDeviceCommand(
		"clear_directive", "Drop the current directive entirely", "clear_directive", nil, "",
	)
	mkDeviceCommand(
		"launch", "Deploy the fleet and start executing the current directive", "launch", nil, "device-launch",
	)
	directiveCmd := mkDeviceCommand(
		"directive", "Update the automation policy for a device", "set_directive",
		[]flagDesc{
			{
				name: "new_directive", short: 'n', required: true, jsonKey: "directive",
			},
			{
				name: "configuration", short: 'c', required: false,
				jsonKey: "configuration", mapFlag: true,
			}}, "",
	)
	mkDeviceCommand(
		"resume", "pick up a stopped directive from where it left off", "resume_directive", nil, "",
	)
	mkDeviceCommand(
		"withdraw", "Recall the fleet and pause execution", "withdraw", nil, "",
	)

	deviceCmd.AddCommand(directiveCmd)
	directiveCmd.AddCommand(deliveryCmd)
	deliveryCmd.Flags().StringP("route", "s", "", "source:dest location codes")
	deliveryCmd.Flags().StringSliceP("resources", "r", []string{}, "resources to collect, type:qty, repeatable")
	deliveryCmd.MarkFlagRequired("route")
	deliveryCmd.MarkFlagRequired("resources")

	directiveCmd.AddCommand(surveyCmd)
	surveyCmd.Flags().BoolP("no_planets", "p", false, "set to skip scanning planets")
	surveyCmd.Flags().BoolP("no_moons", "c", false, "set to skip scanning moons")
	surveyCmd.Flags().BoolP("no_recall", "r", false, "set to not recall the drones once done")

	outputTable["device-launch"] = func(data any) ([]string, [][]string) {
		resp := data.(*models.CommandResp)
		lists := make(map[string][]string)
		for _, cat := range []string{"already_deployed", "deployed", "failed", "skipped"} {
			l := cat[0:1]
			for _, d := range resp.AssignedDevices[cat] {
				a, t := aliasType(d)
				if a != "" {
					d = a
				}
				lists[l] = append(lists[l], strings.Join([]string{d, t}, " "))
			}
			slices.Sort(lists[l])
		}
		return []string{
				"Controller", "Status", "Already deployed", "Deployed", "Failed", "Skipped",
			}, [][]string{{
				resp.DeviceCode.Alias(),
				fmt.Sprintf("%s -> %s", resp.Controller.DirectiveStatusBefore,
					resp.Controller.DirectiveStatusAfter),
				lines(lists["a"]), lines(lists["d"]), lines(lists["f"]), lines(lists["s"]),
			}}
	}
}
