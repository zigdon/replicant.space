package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

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
	resources := []string{"carbon", "conductive", "rares", "silicates",
		"structural", "volatiles"}
	type util struct {
		Actual, Capacity int
	}
	tmpl := "%20s:" + strings.Repeat(" %10s", len(resources))
	titles := []any{"Location"}
	for _, r := range resources {
		if len(r) < 10 {
			titles = append(titles, r)
		} else {
			titles = append(titles, r[:10])
		}
	}
	log(tmpl, titles...)

	update := func(f func(*models.Device), devs ...*models.CodeAlias) {
		var errs []error
		for _, d := range devs {
			dev := &models.Device{Code: d}
			if err := dev.Get(); err != nil {
				errs = append(errs, err)
				continue
			}
			f(dev)
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
			data := []any{ev.Report.Mining.Location}
			for _, r := range resources {
				data = append(data,
					fmt.Sprintf("%d/%d", ev.Report.Mining.Resources[r].Actual,
						ev.Report.Mining.Resources[r].Capacity))
			}
			log(tmpl, data...)
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
				change(
					&d.AttachedDevices,
					slices.DeleteFunc(
						d.AttachedDevices, func(ad *models.Device) bool {
							return ad.Code.String() != d.Code.String()
						}))
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
				change(
					&d.StowedDevices.Devices,
					slices.DeleteFunc(d.StowedDevices.Devices, func(dp *models.DevicePointer) bool {
						return dp.Code.String() != env.DeviceCode.String()
					}))
			}, ev.DeployedFromDeviceCode)
		case "device.stowed":
			ev, err := models.Parse[models.StreamDeviceStowed](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("%s stowed in %s @ %s", env.DeviceCode, ev.StowedIn, env.Location)
			update(func(d *models.Device) {
				change(&d.StowedInDeviceCode, ev.StowedIn)
				change(&d.Location, "")
				change(&d.Status, "stowed")
			}, env.DeviceCode)
			update(func(d *models.Device) {
				if d.StowedDevices.Devices == nil {
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
				log("Consumed: %s (%s)", d.Code.Alias(), d.Code.String())
				_, err := db.DB.Exec("DELETE FROM aliases WHERE designation = $1", d.Code.String())
				errs = append(errs, err)
				_, err = db.DB.Exec("DELETE FROM json_devices WHERE code = $1;", d.Code.String())
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
		case "message.new":
			ev, err := models.Parse[models.StreamMessageNew](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("New message (%s): %s", ev.MessageType, ev.Title)
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
		case "salvage.discovered":
			ev, err := models.Parse[models.StreamSalvageDiscovered](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Salvadge discovered: %s @ %s: %v", ev.Name, ev.Location, ev.Resources)
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
			log("Teleport started: %s online at %s", ev.ReplicantCode, ev.DestinationStar)
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
			nt := new(models.JSONTime)
			update(func(d *models.Device) {
				nt.Set(time.Now().Add(ev.ReturnTime.Duration()))
				change(&d.Travel.Arrives, nt)
				change(&d.Travel.Destination, ev.Origin)
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
			log("Departed from %s to %s: %s", ev.Destination, ev.Origin,
				strings.Join(codeList(append(ev.AttachedDevices, env.DeviceCode)), ", "))

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
