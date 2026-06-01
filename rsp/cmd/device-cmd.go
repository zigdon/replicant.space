package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

type flagDesc struct {
	name string
	short rune
	value string
	desc string
	required bool
	jsonKey string
}

var mkDeviceCommand = func(name, short, command string, flags []flagDesc) {
	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("device")
			data := make(map[string]string)
			var argsFlag flagDesc
			for _, f := range flags {
				if f.name == "" { argsFlag = f }

				val, _ := cmd.Flags().GetString(f.name)
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
						0,
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
							sites, 0)
					}
				}
			}
			return nil
		},
	}
	deviceCmd.AddCommand(cmd)
	for _, f := range flags {
		if f.name == "" { continue }
		if f.short != 0 {
			cmd.Flags().StringP(f.name, string(f.short), f.value, f.desc)
		} else {
			cmd.Flags().String(f.name, f.value, f.desc)
		}
		if f.required {
			cmd.MarkFlagRequired(f.name)
		}
	}
}

func init() {
	mkDeviceCommand(
		"deploy", "Deploy a device", "deploy", nil,
	)
	mkDeviceCommand(
		"directive", "Update the automation policy for a device", "set_directive",
		[]flagDesc{{
			name: "new_directive", short: 'n', required: true, jsonKey: "directive",
		}},
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
		"retarget",  "Change what resource a drone mines", "retarget",
		[]flagDesc{{
			name: "resource", short: 'r', desc: "Resource to mine",
			required: true, jsonKey: "resource_type",
		}},
	)
	mkDeviceCommand(
		"scan", "Initiate a scan of the current location", "scan", nil,
	)
	mkDeviceCommand(
		"search",  "Initiate a search", "search", nil,
	)
	mkDeviceCommand(
		"travel",  "Instruct a device to relocate", "travel",
		[]flagDesc{{
			// args[0]
			required: true, jsonKey: "destination",
		}},
	)
}
