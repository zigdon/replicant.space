package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
)

var mkDeviceCommand = func(name, short, command string, flags []flagDesc, output string) *cobra.Command {
	return mkCommand(deviceCmd, name, short, command, flags, output)
}

func init() {
	mkDeviceCommand(
		"activate", "Activate device (e.g. ftl relay)", "activate", nil, "",
	)
	mkDeviceCommand(
		"adopt", "Add devices to a controller's fleet", "adopt",
		[]flagDesc{{
			name: "adopt", short: 'a', desc: "List of devices to adopt",
			required: true, slice: true, jsonKey: "devices",
		}}, "",
	)
	mkDeviceCommand(
		"attach", "Attach a device (passenger)", "attach",
		[]flagDesc{{
			name: "passenger", short: 'p', desc: "Device to attach",
			required: true, jsonKey: "targets", slice: true,
		}}, "",
	)
	mkDeviceCommand(
		"collect", "Pick up resources at the current location", "collect_resources",
		[]flagDesc{{
			name: "resources", short: 'r', required: true,
			jsonKey: "resources", mapFlag: true,
		}}, "",
	)
	mkDeviceCommand(
		"configure", "Change device configuration", "configure",
		[]flagDesc{{
			name: "taxi_mode", short: 't', required: false, jsonKey: "mode",
		}}, "",
	)
	mkDeviceCommand(
		"deactivate", "Deactivate device (e.g. ftl relay)", "deactivate", nil, "",
	)
	mkDeviceCommand(
		"deploy", "Deploy a device", "deploy", nil, "",
	)
	mkDeviceCommand(
		"deposit", "Drop resources at the current location", "deposit_resources",
		[]flagDesc{{
			name: "resources", short: 'r', required: false,
			jsonKey: "resources", mapFlag: true,
		}}, "",
	)
	mkDeviceCommand(
		"decommission", "Send to the nearest autofactory for deconsturction", "decommission",
		nil, "device-decommission",
	)
	mkDeviceCommand(
		"detach", "Detach a device (passenger)", "detach",
		[]flagDesc{{
			name: "passenger", short: 'p', desc: "Device to detach",
			required: true, jsonKey: "targets", slice: true,
		}}, "",
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
			}}, "",
	)
	mkDeviceCommand(
		"owner", "Change owner of a device", "change_owner",
		[]flagDesc{{
			name: "target", short: 't', desc: "New owner code",
		}}, "",
	)
	mkDeviceCommand(
		"print", "Queue a print job", "enqueue_print",
		[]flagDesc{{
			// args[0]
			desc: "Device to print", required: true, jsonKey: "device_type",
		}, {
			name: "controller", desc: "Controlled to assign after print",
			jsonKey: "controller",
		}, {
			name: "on_complete", short: 'o', mapFlag: true,
			desc:    "Commands to queue when print is done",
			jsonKey: "oncomplete",
		}, {
			name: "repeat", short: 'r', intFlag: true, value: 1,
		}}, "",
	)
	mkDeviceCommand(
		"recall", "Instruct a device to come home and stow itself", "recall", nil, "",
	)
	mkDeviceCommand(
		"release", "Return devices back to direct control", "release",
		[]flagDesc{{
			name: "release", short: 'r', desc: "List of devices to release",
			required: true, slice: true, jsonKey: "devices",
		}}, "",
	)
	mkDeviceCommand(
		"replicate", "Now there are two wubs", "replicate",
		[]flagDesc{{
			name: "target", short: 't', desc: "Replicant to replicate", required: true,
		}}, "",
	)
	mkDeviceCommand(
		"retarget", "Change what resource a drone mines", "retarget",
		[]flagDesc{{
			name: "resource", short: 'r', desc: "Resource to mine",
			required: true, jsonKey: "resource_type",
		}}, "",
	)
	mkDeviceCommand(
		"scan", "Initiate a scan of the current location", "scan", nil, "",
	)
	mkDeviceCommand(
		"search", "Initiate a search", "search", nil, "",
	)
	mkDeviceCommand(
		"stow", "Place a device in the hold of another device", "stow",
		[]flagDesc{{
			name: "target", short: 't', desc: "Device to stow in",
		}}, "",
	)
	mkDeviceCommand(
		"travel", "Instruct a device to relocate", "travel",
		[]flagDesc{{
			// args[0]
			required: true, jsonKey: "destination",
		}}, "",
	)

	outputTable["device-decommission"] = func(data any) ([]string, [][]string) {
		resp, ok := data.(*models.CommandResp)
		if !ok {
			return []string{"Type error"}, [][]string{{fmt.Sprintf("Can't convert %v to CommandResp", data)}}
		}
		return []string{"Status", "Learned", "Recovered Resources"},
	    	[][]string{{resp.Status, resp.BlueprintDiscovered, m(resp.ResourcesRecovered)}}
	}
}
