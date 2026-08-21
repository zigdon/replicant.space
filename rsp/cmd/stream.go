package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var streamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Dip into the event stream, update the cache incrementally",
	RunE:  readStream,
}

func init() {
	rootCmd.AddCommand(streamCmd)
}

func debug[T any](target *T, newVal T) {
	log("%v -> %v", *target, newVal)
	*target = newVal
}

func change[T any](target *T, newVal T) {
	*target = newVal
}

func readStream(cmd *cobra.Command, args []string) error {
	// Load the device network
	relayNetwork := make(map[string]bool)
	rows, err := db.DB.Query(`
		SELECT distinct(location)
		FROM json_devices
		WHERE status = 'relaying'
	`)
	if err != nil {
		return fmt.Errorf("Can't get relay network: %v", err)
	}
	for rows.Next() {
		var loc string
		if err := rows.Scan(&loc); err != nil {
			return fmt.Errorf("Failed to scan locations: %v", err)
		}
		relayNetwork[models.LocationID(loc).Star()] = true
	}
	log("Loaded %d relays", len(relayNetwork))

	const DEBUG = "xxx"
	update := func(f func(*models.Device), devs ...*models.CodeAlias) {
		var errs []error
		for _, d := range devs {
			dev := &models.Device{Code: d}
			if err := dev.Get(); err != nil {
				errs = append(errs, err)
				continue
			}
			before, err := json.MarshalIndent(dev, "", "  ")
			if err != nil {
				panic(err)
			}
			f(dev)
			after, err := json.MarshalIndent(dev, "", "  ")
			if err != nil {
				panic(err)
			}
			if dev.Type == DEBUG {
				log("Changes:")
				log("\n" + cmp.Diff(before, after))
			}
			if err := dev.Cache(); err != nil {
				errs = append(errs, fmt.Errorf("%s: %v", d.Alias(), err))
			}
		}
		if err := errors.Join(errs...); err != nil {
			log("Error applying updates:\n%v", err)
		}
	}

	handle := func(ev map[string]string) error {
		env, err := models.Parse[models.StreamEnvelope]([]byte(ev["data"]))
		if err != nil {
			log("Error parsing %v: %v", ev, err)
			return err
		}
		log := func(tmpl string, args ...any) {
			common.TimeLog(env.Created.Time(), tmpl, args...)
		}
		payload, err := json.Marshal(env.Payload)
		if err != nil {
			log("Can't remarshal payload: %v", err)
			return err
		}
		switch env.Event {
		case "ami.adopted":
			ev, err := models.Parse[models.StreamAmiAdopted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			var ids []*models.CodeAlias
			var cds []*models.ControlledDevice
			for _, d := range ev.Devices {
				ids = append(ids, d.Code)
				cds = append(cds, &models.ControlledDevice{
					Code:     d.Code,
					Type:     d.Type,
					Location: env.Location,
					Status:   "idle",
				})
			}
			log("AMI %s adoptd %d devices: %v", env.DeviceCode.Alias(), len(ev.Devices), ids)
			update(func(d *models.Device) {
				change(&d.ControlledDevices, append(d.ControlledDevices, cds...))
			}, env.DeviceCode)
			update(func(d *models.Device) {
				change(&d.ControllerDeviceCode, env.DeviceCode)
			}, ids...)
		case "ami.launched":
			ev, err := models.Parse[models.StreamAmiLaunched](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("AMI %s launched: %s, %d devices deployed",
				env.DeviceCode.Alias(), ev.DirectiveStatus, ev.DevicesDeployed)
		case "ami.mining.digest":
			ev, err := models.Parse[models.StreamAmiDigest](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			/*
				data := []any{ev.Report.Mining.Location}
				for _, r := range resources {
					data = append(data,
						fmt.Sprintf("%d/%d", ev.Report.Mining.Resources[r].Actual,
							ev.Report.Mining.Resources[r].Capacity))
				}
				log(tmpl, data...)
			*/
			devBySt := make(map[string][]*models.CodeAlias)
			for _, d := range ev.Devices {
				if devBySt[d.Status] == nil {
					devBySt[d.Status] = []*models.CodeAlias{}
				}
				devBySt[d.Status] = append(devBySt[d.Status], d.DeviceCode)
			}
			for k, v := range devBySt {
				update(func(d *models.Device) {
					change(&d.Status, k)
					change(&d.Location, models.LocationID(ev.Report.Mining.Location))
				}, v...)
			}
		case "ami.released":
			ev, err := models.Parse[models.StreamAmiReleased](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			ids := make([]*models.CodeAlias, len(ev.Devices))
			for n := range ev.Devices {
				ids[n] = ev.Devices[n].Code
			}
			update(func(d *models.Device) {
				change(&d.ControllerDeviceCode, nil)
			}, ids...)
			update(func(d *models.Device) {
				var cd []*models.ControlledDevice
				for _, dev := range d.ControlledDevices {
					if dev.Code.Contained(ids) {
						continue
					}
					cd = append(cd, dev)
				}
				change(&d.ControlledDevices, cd)
			}, env.DeviceCode)
			log("%s released %d devices: %s", env.DeviceCode, len(ids), ids)
		case "ami.survey.digest":
			ev, err := models.Parse[models.StreamAmiDigest](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			for _, nd := range ev.Devices {
				update(func(d *models.Device) {
					change(&d.Status, nd.Status)
					change(&d.Location, env.Location)
				}, nd.DeviceCode)
			}
		case "ami.transport.digest":
			ev, err := models.Parse[models.StreamAmiDigest](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			st := make(map[string][]*models.CodeAlias)
			for _, d := range ev.Devices {
				st[d.Status] = append(st[d.Status], d.DeviceCode)
			}
			var cnts []string
			for k, v := range st {
				cnts = append(cnts, fmt.Sprintf("%s (%d)", k, len(v)))
				update(func(d *models.Device) {
					change(&d.Status, k)
				}, v...)
			}
			slices.Sort(cnts)
			log("Updating transports status: %s", strings.Join(cnts, ", "))
		case "blueprint.unlocked":
			ev, err := models.Parse[models.StreamBlueprintUnlocked](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Blueprint unlocked: %s - %s", ev.DeviceType, ev.ShortDescription)
			if _, err := rest.Blueprints(true); err != nil {
				log("ERROR refreshing blueprints: %v", err)
			}
		case "bobnet.new":
			ev, err := models.Parse[models.StreamBobnetNew](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("%s: <%s> %s", ev.Channel, ev.ReplicantName, ev.Message)
		case "device.attached":
			ev, err := models.Parse[models.StreamDeviceAttached](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			update(func(d *models.Device) {
				change(&d.AttachedDevices, append(d.AttachedDevices, &models.Device{
					Code: ev.TargetCode,
					Type: ev.TargetType,
				}))
			}, env.DeviceCode)
			update(func(d *models.Device) {
				change(&d.AttachedToDeviceCode, env.DeviceCode)
			}, ev.TargetCode)
		case "device.changed_owner":
			ev, err := models.Parse[models.StreamDeviceChangedOwner](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Ownership of %s changed: %s -> %s", env.DeviceCode, ev.FromReplicant, ev.ToReplicant)
			update(func(d *models.Device) {
				if d.Owner == nil {
					return
				}
				change(&d.Owner.Code, ev.ToReplicant)
			}, env.DeviceCode)
		case "device.decommissioned":
			ev, err := models.Parse[models.StreamDeviceDecommissioned](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Device decomissioned: %s @ %s: %s", env.DeviceCode, env.Location, ev.BlueprintDiscovered)
			if ev.BlueprintDiscovered != "" {
				if _, err := rest.Blueprints(true); err != nil {
					log("Error loading new blueprint %q: %v", ev.BlueprintDiscovered, err)
				}
			}
		case "device.detached":
			ev, err := models.Parse[models.StreamDeviceDetached](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			update(func(d *models.Device) {
				ads := slices.DeleteFunc(d.AttachedDevices, func(ad *models.Device) bool {
					return ad.Code.String() != d.Code.String()
				})
				change(&d.AttachedDevices, ads)
			}, env.DeviceCode)
			update(func(d *models.Device) {
				change(&d.AttachedToDeviceCode, nil)
			}, ev.TargetCode)
			log("Devices detached from %s: %v", env.DeviceCode, ev.TargetCode)
		case "device.deployed":
			ev, err := models.Parse[models.StreamDeviceDeployed](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("%s deployed from %s @ %s", env.DeviceCode, ev.DeployedFromDeviceCode, env.Location)
			update(func(d *models.Device) {
				change(&d.StowedInDeviceCode, nil)
				change(&d.Location, env.Location)
				if d.Type == "ftl_beacon" {
					change(&d.Status, "monitoring")
				} else {
					change(&d.Status, "idle")
				}
			}, env.DeviceCode)
			update(func(d *models.Device) {
				if d.StowedDevices != nil {
					sds := slices.DeleteFunc(d.StowedDevices.Devices,
						func(dp *models.DevicePointer) bool {
							return dp.Code.String() == env.DeviceCode.String()
						})
					change(&d.StowedDevices.Devices, sds)
				}
			}, ev.DeployedFromDeviceCode)
		case "device.stowed":
			ev, err := models.Parse[models.StreamDeviceStowed](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			if ev.StowedIn == nil {
				return fmt.Errorf("device.stowed error:\npayload=%s\nenv=%v\nev=%v", payload, env, ev)
			}
			log("%s stowed in %s @ %s", env.DeviceCode, ev.StowedIn, env.Location)
			update(func(d *models.Device) {
				change(&d.StowedInDeviceCode, ev.StowedIn)
				change(&d.Location, "")
				change(&d.Status, "stowed")
			}, env.DeviceCode)
			update(func(d *models.Device) {
				if d.StowedDevices == nil || d.StowedDevices.Devices == nil {
					return
				}
				if !slices.ContainsFunc(d.StowedDevices.Devices, func(dp *models.DevicePointer) bool {
					return dp.Code.String() == env.DeviceCode.String()
				}) {
					change(
						&d.StowedDevices.Devices,
						append(d.StowedDevices.Devices, &models.DevicePointer{
							Code: env.DeviceCode,
							Type: env.DeviceType,
						}))
				}
			}, ev.StowedIn)
		case "device.unfurled":
			log("%s unfurled at %s", env.DeviceCode, env.Location)
			update(func(d *models.Device) {
				change(&d.Status, "idle")
			}, env.DeviceCode)
		case "diversion.activated":
			ev, err := models.Parse[models.StreamDiversionActivated](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Diversion activated: %s", ev.ObjectDesignation)
			update(func(d *models.Device) {
				change(&d.Location, ev.ObjectDesignation)
				change(&d.Status, "diverting")
			}, env.DeviceCode)
		case "directive.completed":
			ev, err := models.Parse[models.StreamDirectiveCompleted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Directive complete: %s - %s", env.DeviceCode.Alias(), ev.Directive)
		case "diversion.diverted":
			ev, err := models.Parse[models.StreamDiversionDiverted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			devs, err := rest.Devices(map[string]string{
				"location":    string(ev.ObjectDesignation),
				"device_type": "propulsor",
			})
			if err != nil {
				log("Can't get props at %s: %v", ev.ObjectDesignation, err)
				return err
			}
			var ids []*models.CodeAlias
			for _, d := range devs {
				ids = append(ids, d.Code)
			}
			log("Diversion complete %s: %v", ev.ObjectDesignation, codeList(ids))
			update(func(d *models.Device) {
				change(&d.Location, ev.ObjectDesignation)
				change(&d.Status, "idle")
			}, ids...)
		case "diversion.deactivated":
			_, err := models.Parse[models.StreamDiversionDeactivated](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			update(func(d *models.Device) {
				change(&d.Status, "idle")
			}, env.DeviceCode)
		case "diversion.wear":
			ev, err := models.Parse[models.StreamDiversionWear](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			update(func(d *models.Device) {
				change(&d.OperationalCapacity, ev.OperationalCapacity)
			}, env.DeviceCode)
			log("Wear on %s, operational capacity: %.0f%%", env.DeviceCode.Alias(), ev.OperationalCapacity)
		case "directive.resumed":
			ev, err := models.Parse[models.StreamDirectiveResumed](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Directive %q resumed on %s",
				ev.Directive, env.DeviceCode.Alias())
			update(func(d *models.Device) {
				if d.AmiDirective == nil {
					return
				}
				change(&d.AmiDirective.Name, ev.Directive)
			})
		case "directive.set":
			ev, err := models.Parse[models.StreamDirectiveSet](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Directive %q set on %s: %v",
				ev.Directive, env.DeviceCode.Alias(), ev.Configuration)
			update(func(d *models.Device) {
				if d.AmiDirective == nil {
					return
				}
				change(&d.AmiDirective.Name, ev.Directive)
				change(&d.AmiDirective.Config, ev.Configuration)
			})
		case "event.completed":
			ev, err := models.Parse[models.StreamEventCompleted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Event complete: %s - %s => %v", ev.Designation, ev.EventType, ev.Rewards)
			var errs []error
			for _, d := range ev.Consumed.Devices {
				_, err := db.DB.Exec("DELETE FROM aliases WHERE designation = $1", d.Code.String())
				errs = append(errs, err)
				_, err = db.DB.Exec("DELETE FROM json_devices WHERE code = $1;", d.Code.String())
				errs = append(errs, err)
			}
			for _, d := range ev.Rewards.Devices {
				_, err := rest.DeviceInfo(d.Code)
				errs = append(errs, err)
			}
			if err := errors.Join(errs...); err != nil {
				log("Error removing devices from the cache: %v", err)
			}
		case "experience.gained":
			ev, err := models.Parse[models.StreamExperienceGained](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("XP gained: %d for %s at %s", ev.Amount, ev.Source, env.Location)
		case "hub.activated":
			ev, err := models.Parse[models.StreamHubActivated](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Hub %s activated at %s by %s", env.DeviceCode, ev.Location, env.ReplicantCode)
			update(func(d *models.Device) {
				change(&d.Status, "relaying")
			}, env.DeviceCode)
			relayNetwork[ev.Location.Star()] = true
		case "hub.maintained":
			ev, err := models.Parse[models.StreamHubMaintained](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("%s @ %s (%.0f%%) consumed for maintanence: %v",
				env.DeviceCode.Alias(), env.Location, ev.Capacity, ev.ResourcesConsumed)
			update(func(d *models.Device) {
				change(&d.OperationalCapacity, ev.Capacity)
			}, env.DeviceCode)
			for k, v := range ev.ResourcesConsumed {
				ev.ResourcesConsumed[k] = -v
			}
			if err := db.UpdateInventory(false, string(env.Location), ev.ResourcesConsumed); err != nil {
				log("Error updating inventory: %v", err)
			}
		case "message.new":
			ev, err := models.Parse[models.StreamMessageNew](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("New message (%s): %s", ev.MessageType, ev.Title)
		case "mining.started":
			ev, err := models.Parse[models.StreamMiningStarted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("%s started mining %s at %s", env.DeviceCode, ev.ResourceType, ev.Site)
			update(func(d *models.Device) {
				change(&d.Status, fmt.Sprintf("mining (%s)", ev.ResourceType))
				change(&d.Location, env.Location)
			}, env.DeviceCode)
		case "mining.stopped":
			ev, err := models.Parse[models.StreamMiningStopped](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("%s mined %d %s at %s",
				env.DeviceCode, ev.QuantityMined, ev.ResourceType, env.Location)
			update(func(d *models.Device) {
				change(&d.Status, "idle")
				change(&d.Location, env.Location)
			}, env.DeviceCode)
		case "multiplayer.replicant_entered":
			ev, err := models.Parse[models.StreamMultiplayerReplicantEntered](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("%s entered %s", ev.ReplicantName, ev.Star)
		case "multiplayer.replicant_left":
			ev, err := models.Parse[models.StreamMultiplayerReplicantLeft](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("%s left %s", ev.ReplicantName, ev.Star)
		case "print.completed":
			ev, err := models.Parse[models.StreamPrintCompleted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			// Create an alias for this device
			alias, err := db.Alias(ev.NewDeviceCode.String(), ev.DeviceType)
			if err != nil {
				log("Error creating a new alias for %s (%s): %v",
					ev.NewDeviceCode.String(), ev.DeviceType, err)
			}
			log("%s finished printing %s at %s: %s (%s)",
				env.DeviceCode, ev.DeviceType, env.Location, alias, ev.NewDeviceCode.String())
			update(func(d *models.Device) {
				if len(d.PrintQueue) > 0 {
					pq := d.PrintQueue[0]
					change(&d.Printing, &models.DevicePrint{
						Completes: new(models.JSONTime).Set(
							time.Now().Add(common.GetBP(pq.Type).PrintTime.Duration())),
						Eta:        common.GetBP(pq.Type).PrintTime,
						DeviceType: pq.Type,
						Started:    new(models.JSONTime).Set(time.Now()),
						Tags:       pq.Tags,
					})
					change(&d.PrintQueue, d.PrintQueue[1:])
				} else {
					change(&d.Status, "idle")
				}
			}, env.DeviceCode)
		case "print.started":
			ev, err := models.Parse[models.StreamPrintStarted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			// ev.DeviceType
			update(func(d *models.Device) {
				change(&d.Status, "printing")
				change(&d.Printing, &models.DevicePrint{
					Completes:  ev.Completes,
					DeviceType: ev.DeviceType,
					Started:    env.Created,
				})
			}, env.DeviceCode)
			log("Printing %s at %s: ETA %s", ev.DeviceType, env.DeviceCode, ev.Completes)
		case "relay.activated":
			_, err := models.Parse[models.StreamRelayActivated](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Relay %s activated at %s", env.DeviceCode, env.Location)
			update(func(d *models.Device) {
				change(&d.Status, "relaying")
				change(&d.Location, env.Location)
			}, env.DeviceCode)
			relayNetwork[env.Location.Star()] = true
		case "salvage.discovered":
			ev, err := models.Parse[models.StreamSalvageDiscovered](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Salvage discovered: %s @ %s: %v", ev.Name, ev.Location, ev.Resources)
		case "site.depleted":
			ev, err := models.Parse[models.StreamSiteDepleted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			res, err := rest.Location(string(env.Location))
			if err != nil {
				log("Error getting location details: %v", err)
				return err
			}
			var out []string
			for _, i := range res.Inventory {
				out = append(out, fmt.Sprintf("%d %s", i.Quantity, i.ResourceType[:2]))
			}
			log("%s depleted: %s", ev.Site, strings.Join(out, ", "))
		case "teleport.completed":
			ev, err := models.Parse[models.StreamTeleportCompleted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Teleport complete: %s online at %s", ev.ReplicantCode, ev.DestinationStar)
		case "teleport.started":
			ev, err := models.Parse[models.StreamTeleportStarted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Teleport started: %s (%s -> %s)", ev.ReplicantCode, ev.SourceStar, ev.DestinationStar)
		case "trade.completed":
			ev, err := models.Parse[models.StreamTradeCompleted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Trade complete: %s @ %s: ?? -> %v", ev.TradeName, env.Location, ev.RewardsReceived)
			var errs []error
			_, err = rest.Location(string(env.Location))
			errs = append(errs, err)
			for _, d := range ev.RewardsReceived.Devices {
				_, err = rest.DeviceInfo(d)
				errs = append(errs, err)
			}
			return errors.Join(errs...)
		case "transport.collected":
			ev, err := models.Parse[models.StreamTransportCollected](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("%s collected @ %s: %v", env.DeviceCode.Alias(), env.Location, ev.Resources)
			update(func(d *models.Device) {
				inv := d.Cargo
				for _, i := range inv {
					i.Quantity += ev.Resources[i.ResourceType]
				}
				change(&d.Cargo, inv)
			}, env.DeviceCode)
			for k := range ev.Resources {
				ev.Resources[k] *= -1
			}
			if err := db.UpdateInventory(false, string(env.Location), ev.Resources); err != nil {
				log("Error updating inventory: %v", err)
			}
		case "transport.delivered":
			ev, err := models.Parse[models.StreamTransportDelivered](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("%s delivered @ %s: %v", env.DeviceCode.Alias(), env.Location, ev.Resources)
			update(func(d *models.Device) {
				inv := d.Cargo
				var newInv []*models.Inventory
				for _, i := range inv {
					i.Quantity -= ev.Resources[i.ResourceType]
					if i.Quantity > 0 {
						newInv = append(newInv, i)
					}
				}
				change(&d.Cargo, newInv)
			}, env.DeviceCode)
			if err := db.UpdateInventory(false, string(env.Location), ev.Resources); err != nil {
				log("Error updating inventory: %v", err)
			}
		case "travel.arrived":
			ev, err := models.Parse[models.StreamTravelArrived](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			devs := []*models.CodeAlias{env.DeviceCode}
			devs = append(devs, ev.AttachedDevices...)
			log("Arrived at %s from %s: %s", ev.Destination, ev.Origin, strings.Join(codeList(devs), ", "))
			update(func(d *models.Device) {
				change(&d.Location, ev.Destination)
				change(&d.Travel, nil)
				change(&d.InControlRange, relayNetwork[ev.Destination.Star()])
				change(&d.Status, "idle")
			}, devs...)
		case "travel.cancelled":
			ev, err := models.Parse[models.StreamTravelCancelled](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Travel canceled: %q aborted travel to %q, returning to %q in %s",
				env.DeviceCode, ev.Destination, ev.Origin, ev.ReturnTime)
			devs := append(ev.AttachedDevices, env.DeviceCode)
			update(func(d *models.Device) {
				if nt := d.Travel; nt != nil {
					nt.Arrives.Set(time.Now().Add(ev.ReturnTime.Duration()))
					nt.Destination = ev.Origin
					change(&d.Travel, nt)
				}
			}, devs...)
		case "travel.departed":
			ev, err := models.Parse[models.StreamTravelDeparted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			var legs []*models.TripLeg
			orig := ev.Origin
			for _, l := range ev.Legs {
				legs = append(legs, &models.TripLeg{
					From: orig,
					To:   l.To,
					Type: l.Type,
				})
				orig = l.To
			}
			update(func(d *models.Device) {
				change(&d.Travel, &models.Trip{
					Arrives:     ev.ArrivesAt,
					Destination: ev.Destination,
					Origin:      ev.Origin,
					TotalTime:   ev.TravelTime,
					Type:        ev.TravelType,
					Route:       legs,
				})
				change(&d.Location, "")
				if ev.TravelType == "surge" {
					change(&d.Status, "surging")
				} else {
					change(&d.Status, "travelling")
				}
			}, env.DeviceCode)
			log("Departed to %s from %s: %s", ev.Destination, ev.Origin,
				strings.Join(codeList(append(ev.AttachedDevices, env.DeviceCode)), ", "))

		// Next case here

		/*
			ev, err := models.Parse[models.StreamTravelDeparted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
		*/

		default:
			log("Unknown event type: %q", ev["event"])
			prettyPrint(ev)
		}
		return nil
	}

	var lastEvent string
	if len(args) > 0 {
		lastEvent = args[0]
	} else {
		row := db.DB.QueryRow("SELECT eventid FROM event_stream ORDER BY id desc LIMIT 1")
		if err := row.Scan(&lastEvent); err != nil {
			return err
		}
	}
	if err := rest.ReadStream(lastEvent, handle); err != nil {
		return err
	}

	return nil
}
