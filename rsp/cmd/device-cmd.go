package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
)

func mkDeviceCommand[T any](name, short, command string, flags []flagDesc, output string) *cobra.Command {
	return mkCommand[T](deviceCmd, name, short, command, flags, output)
}

func init() {
	mkDeviceCommand[models.CommandResp](
		"activate", "Activate device (e.g. ftl relay)", "activate", nil, "",
	)
	mkDeviceCommand[models.CommandResp](
		"adopt", "Add devices to a controller's fleet", "adopt",
		[]flagDesc{{
			name: "target", short: 't', desc: "List of devices to adopt",
			required: true, slice: true, jsonKey: "devices", rangeFlag: true,
		}}, "device-adopt",
	)
	mkDeviceCommand[models.CommandResp](
		"attach", "Attach a device (passenger)", "attach",
		[]flagDesc{{
			name: "target", short: 't', desc: "Device to attach",
			required: true, jsonKey: "targets", slice: true, rangeFlag: true,
		}}, "device-attach",
	)
	mkDeviceCommand[models.CommandResp](
		"collect", "Pick up resources at the current location", "collect_resources",
		[]flagDesc{{
			name: "resources", short: 'r', required: true,
			jsonKey: "resources", mapFlag: true,
		}}, "",
	)
	mkDeviceCommand[models.CommandResp](
		"compact", "Prepare a modular device for transport", "compact", nil, "",
	)
	mkDeviceCommand[models.CommandResp](
		"configure", "Change device configuration", "configure",
		[]flagDesc{{
			name: "taxi_mode", short: 't', required: false, jsonKey: "mode",
		}}, "",
	)
	mkDeviceCommand[models.CommandResp](
		"deactivate", "Deactivate device (e.g. ftl relay)", "deactivate", nil, "",
	)
	mkDeviceCommand[models.CommandResp](
		"deploy", "Deploy a device", "deploy", nil, "",
	)
	mkDeviceCommand[models.CommandResp](
		"deposit", "Drop resources at the current location", "deposit_resources",
		[]flagDesc{{
			name: "resources", short: 'r', required: false,
			jsonKey: "resources", mapFlag: true,
		}}, "",
	)
	mkDeviceCommand[models.CommandResp](
		"decommission", "Send to the nearest autofactory for deconsturction", "decommission",
		nil, "device-decommission",
	)
	mkDeviceCommand[models.CommandResp](
		"detach", "Detach a device (passenger)", "detach",
		[]flagDesc{{
			name: "target", short: 't', desc: "Device to detach",
			jsonKey: "targets", slice: true, rangeFlag: true,
		}}, "device-attach",
	)
	mkDeviceCommand[models.CommandResp](
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
	mkDeviceCommand[models.CommandResp](
		"owner", "Change owner of a device", "change_owner",
		[]flagDesc{{
			name: "target", short: 't', desc: "New owner code",
		}}, "",
	)
	mkDeviceCommand[models.CommandResp](
		"clear_queue", "Clear print queue", "clear_queue", nil, "",
	)
	mkDeviceCommand[models.CommandResp](
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
			name: "repeat", short: 'r', intFlag: true,
			desc: "Enqueue multiple copies",
		}}, "",
	)
	mkDeviceCommand[models.CommandResp](
		"recall", "Instruct a device to come home and stow itself", "recall", nil, "",
	)
	mkDeviceCommand[models.CommandResp](
		"release", "Return devices back to direct control", "release",
		[]flagDesc{{
			name: "target", short: 't', desc: "List of devices to release",
			required: true, slice: true, jsonKey: "devices", rangeFlag: true,
		}}, "device-adopt",
	)
	mkDeviceCommand[models.CommandResp](
		"replicate", "Now there are two wubs", "replicate",
		[]flagDesc{{
			name: "target", short: 't', desc: "Replicant to replicate", required: true,
		}}, "",
	)
	mkDeviceCommand[models.CommandResp](
		"retarget", "Change what resource a drone mines", "retarget",
		[]flagDesc{{
			name: "resource", short: 'r', desc: "Resource to mine",
			required: true, jsonKey: "resource_type",
		}}, "",
	)
	mkDeviceCommand[models.CommandResp](
		"scan", "Initiate a scan of the current location", "scan", nil, "",
	)
	mkDeviceCommand[models.CommandResp](
		"search", "Initiate a search", "search", nil, "",
	)
	mkDeviceCommand[models.CommandResp](
		"stow", "Place a device in the hold of another device", "stow",
		[]flagDesc{{
			name: "target", short: 't', desc: "Device to stow in",
		}}, "device-stow",
	)
	mkDeviceCommand[models.CommandResp](
		"travel", "Instruct a device to relocate", "travel",
		[]flagDesc{{
			// args[0]
			required: true, jsonKey: "destination",
		}, {
			name: "dry_run", boolFlag: true, jsonKey: "dry_run",
		}}, "device-travel",
	)
	mkDeviceCommand[models.CommandResp](
		"unfurl", "Reassemble a modular device after transport", "unfurl", nil, "",
	)

	// Output tables
	outputTable["device-stow"] = func(data any) ([]string, [][]string) {
		resp, ok := data.(*models.CommandResp)
		if !ok {
			return []string{"Type error"}, [][]string{{fmt.Sprintf("Can't convert %v to CommandResp", data)}}
		}
		return []string{"Device", "Status", "Stowed in"}, [][]string{{
			resp.DeviceCode.Alias(), resp.Status, resp.StowedIn.Alias(),
		}}
	}

	outputTable["device-adopt"] = func(data any) ([]string, [][]string) {
		resp, ok := data.(*models.CommandResp)
		if !ok {
			return []string{"Type error"}, [][]string{{fmt.Sprintf("Can't convert %v to CommandResp", data)}}
		}
		var as, rs []string
		for _, d := range resp.AdoptedDevices {
			as = append(as, d.Code.Alias())
		}
		for _, d := range resp.Released {
			rs = append(rs, d.Code.Alias())
		}
		return []string{"Controller", "Status", "Adopted", "Released"}, [][]string{{
			resp.ControllerCode.Alias(), resp.Status, list(as), list(rs),
		}}
	}
	outputTable["device-attach"] = func(data any) ([]string, [][]string) {
		// The response is _either_ CommandResp (if we provided a list of
		// targets) or CommandDetachAll when targets = null. I have Opinions.
		resp, ok := data.(*models.CommandResp)
		var as, rs []string
		if ok {
			for _, d := range resp.Attached {
				as = append(as, d.Code.Alias())
			}
			for _, d := range resp.Detached {
				rs = append(rs, d.Code.Alias())
			}
		} else {
			cda, ok := data.(*models.CommandDetachAll)
			if !ok {
				return []string{"Type error"}, [][]string{{
					fmt.Sprintf(
						"Can't convert %v to CommandResp or CommandDetachAll", data)}}
			}
			rs = cda.Detached
		}
		return []string{"Controller", "Status", "Attached", "Detached"}, [][]string{{
			resp.ControllerCode.Alias(), resp.Status, list(as), list(rs),
		}}

	}
	outputTable["device-decommission"] = func(data any) ([]string, [][]string) {
		resp, ok := data.(*models.CommandResp)
		if !ok {
			return []string{"Type error"}, [][]string{{fmt.Sprintf("Can't convert %v to CommandResp", data)}}
		}
		return []string{"Status", "Learned", "Recovered Resources"},
			[][]string{{resp.Status, resp.BlueprintDiscovered, m(resp.ResourcesRecovered)}}
	}
	outputTable["device-travel"] = func(data any) ([]string, [][]string) {
		resp, ok := data.(*models.CommandResp)
		if !ok {
			return []string{"Type error"}, [][]string{{fmt.Sprintf("Can't convert %v to CommandResp", data)}}
		}
		var origin, dest []string
		origin = append(origin, resp.Origin)
		origin = append(origin, t(resp.Departed.Time()))
		dest = append(dest, resp.Destination)
		dest = append(dest, t(resp.Arrives.Time()))
		return []string{"Status", "Departed", "Destination", "Total Time"},
			[][]string{{resp.Status, lines(origin), lines(dest), resp.TotalTime.String()}}
	}
}
