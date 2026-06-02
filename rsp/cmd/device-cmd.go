package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

type flagDesc struct {
	name     string
	short    rune
	value    string
	desc     string
	required bool
	slice    bool
	jsonKey  string
	mapFlag  bool
}

var mkDeviceCommand = func(name, short, command string, flags []flagDesc) {
	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("device")
			data := make(map[string]any)
			var argsFlag flagDesc
			for _, f := range flags {
				if f.name == "" {
					argsFlag = f
				}

				var val any
				if f.slice {
					val, _ = cmd.Flags().GetStringSlice(f.name)
				} else if f.mapFlag {
					dataMap := make(map[string]string)
					ms, _ := cmd.Flags().GetStringSlice(f.name)
					for _, mv := range ms {
						bits := strings.Split(mv, ":")
						dataMap[bits[0]] = bits[1]
					}
					val = dataMap
				} else {
					val, _ = cmd.Flags().GetString(f.name)
				}
				if f.required {
					data[f.jsonKey] = val
				} else if val != "" {
					data[f.jsonKey] = val
				}
			}
			if argsFlag.jsonKey != "" {
				if len(args) == 0 || args[0] == "" {
					return fmt.Errorf("Argument is required for %q", name)
				}
				data[argsFlag.jsonKey] = args[0]
			}
			resp, err := rest.DeviceCommand(id, command, data)
			if err != nil {
				return fmt.Errorf("Error sending %q to %q: %v", command, id, err)
			}
			if raw, _ := cmd.Flags().GetBool("raw"); raw {
				prettyPrint(resp)
			} else {
				if resp.JsonErr == "" {
					printTable(
						[]string{
							"Code", "Location", "Star", "Belt", "Status",
							"ETA", "Started", "Ends"},
						[][]string{{
							resp.DeviceCode, resp.Location, resp.Star, resp.Belt,
							resp.Status, resp.EtaSeconds.String(), resp.StartedAt,
							resp.CompletesAt,
						}},
					)
				} else {
					log("error: %v", resp.JsonErr)
					if len(resp.AvailableSites) > 0 {
						var sites [][]string
						for _, s := range resp.AvailableSites {
							sites = append(sites, []string{
								s.Designation, s.Name, s.SalvageType,
							})
						}
						printTable([]string{"Designation", "Name", "SalvageType"},
							sites)
					}
				}
			}
			return nil
		},
	}
	deviceCmd.AddCommand(cmd)
	for _, f := range flags {
		if f.name == "" {
			continue
		}
		if f.slice || f.mapFlag {
			if f.short != 0 {
				cmd.Flags().StringSliceP(f.name, string(f.short), []string{f.value}, f.desc)
			} else {
				cmd.Flags().StringSlice(f.name, []string{f.value}, f.desc)
			}
		} else {
			if f.short != 0 {
				cmd.Flags().StringP(f.name, string(f.short), f.value, f.desc)
			} else {
				cmd.Flags().String(f.name, f.value, f.desc)
			}
		}
		if f.required {
			cmd.MarkFlagRequired(f.name)
		}
	}
}

func init() {
	mkDeviceCommand(
		"assemble", "Bring the fleet home to the controller's current location without ending the directive", "assemble", nil,
	)
	mkDeviceCommand(
		"adopt", "Add devices to a controller's fleet", "adopt",
		[]flagDesc{{
			name: "adopt", short: 'a', desc: "List of devices to adopt",
			required: true, slice: true, jsonKey: "devices",
		}},
	)
	mkDeviceCommand(
		"deploy", "Deploy a device", "deploy", nil,
	)
	mkDeviceCommand(
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
		"mine", "Instruct a drone to start mining", "start_mining",
		[]flagDesc{{
			name: "resource", short: 'r', desc: "Resource to mine",
			required: true, jsonKey: "resource_type",
		},
			{
				name: "location", short: 'l',
				desc: "Specific location to mine", jsonKey: "location",
			}},
	)
	mkDeviceCommand(
		"retarget", "Change what resource a drone mines", "retarget",
		[]flagDesc{{
			name: "resource", short: 'r', desc: "Resource to mine",
			required: true, jsonKey: "resource_type",
		}},
	)
	mkDeviceCommand(
		"release", "Return devices back to direct control", "release",
		[]flagDesc{{
			name: "release", short: 'r', desc: "List of devices to release",
			required: true, slice: true, jsonKey: "devices",
		}},
	)
	mkDeviceCommand(
		"scan", "Initiate a scan of the current location", "scan", nil,
	)
	mkDeviceCommand(
		"search", "Initiate a search", "search", nil,
	)
	mkDeviceCommand(
		"travel", "Instruct a device to relocate", "travel",
		[]flagDesc{{
			// args[0]
			required: true, jsonKey: "destination",
		}},
	)
	mkDeviceCommand(
		"withdraw", "Recall the fleet and pause execution", "withdraw", nil,
	)
}
