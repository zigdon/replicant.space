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
			_, err := models.Parse[models.StreamAmiDigest](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Ignoring %s", env.Event)
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
		case "diversion.diverted":
			ev, err := models.Parse[models.StreamDiversionDiverted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("Diversion complete: %s", ev.ObjectDesignation)
			update(func(d *models.Device) {
				change(&d.Location, ev.ObjectDesignation)
				change(&d.Status, "idle")
			}, env.DeviceCode)
		case "diversion.deactivated":
			_, err := models.Parse[models.StreamDiversionDeactivated](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			update(func(d *models.Device) {
				change(&d.Status, "idle")
			}, env.DeviceCode)
		case "experience.gained":
			ev, err := models.Parse[models.StreamExperienceGained](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("XP gained: %d for %s at %s", ev.Amount, ev.Source, env.Location)
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
					debug(&d.Printing, &models.DevicePrint{
						Completes: new(models.JSONTime).Set(
							time.Now().Add(common.GetBP(pq.Type).PrintTime.Duration())),
						Eta:        common.GetBP(pq.Type).PrintTime,
						DeviceType: pq.Type,
						Started:    new(models.JSONTime).Set(time.Now()),
						Tags:       pq.Tags,
					})
					debug(&d.PrintQueue, d.PrintQueue[1:])
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
		case "site.depleted":
			ev, err := models.Parse[models.StreamSiteDepleted](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("%s depleted", ev.Site)
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
		default:
			log("Unknown event type: %q", ev["event"])
			prettyPrint(ev)
		}
		return nil
	}

	if err := rest.ReadStream(handle); err != nil {
		return err
	}

	return nil
}
