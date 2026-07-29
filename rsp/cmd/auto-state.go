package cmd

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/auto"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

func autoState(cmd *cobra.Command, args []string) error {
	// If devices were specified, process those. Otherwise, loop over
	// all the defined states
	var devs []*models.Device
	if len(args) == 0 {
		var err error
		res, err := rest.GetTagged("auto")
		if err != nil {
			return err
		}
		for _, d := range res.Devices {
			i, err := getInfo(d.Code)
			if err != nil {
				return err
			}
			devs = append(devs, i)
		}
	} else {
		for _, d := range args {
			i, err := getInfo(models.NewCodeAlias(d))
			if err != nil {
				return err
			}
			devs = append(devs, i)
		}
	}
	if len(devs) == 0 {
		return fmt.Errorf("No devices tagged 'auto' found.")
	}

	var sms = make(map[*models.CodeAlias]auto.Machine)
	dryRun := getBool(cmd, "dry_run")
	for _, d := range devs {
		if slices.Contains(d.Tags, "auto:prospect") {
			log("%s: prospect -> %v", d.Code.Alias(), d.Tags)
			sms[d.Code] = &auto.ProspectMachine{}
		} else if slices.Contains(d.Tags, "auto:relay") {
			log("%s: relay -> %v", d.Code.Alias(), d.Tags)
			sms[d.Code] = &auto.RelayMachine{}
		} else {
			return fmt.Errorf("Unknown state machine for %q: %v", d.Code.Alias(), d.Tags)
		}
		if err := sms[d.Code].Start(d, dryRun); err != nil {
			return err
		}
	}

	eq := auto.NewEventQueue(5 * time.Minute)
	var errs []error
	var runStep func(d *models.CodeAlias, m auto.Machine) error
	runStep = func(d *models.CodeAlias, m auto.Machine) error {
		t, err := m.Process()
		if err != nil {
			errs = append(errs, err)
		} else if t.IsZero() {
			errs = append(errs, fmt.Errorf("%s: No time for next step", d.Alias()))
		} else {
			eq.AddEvent(
				d.Alias(),
				fmt.Sprintf("%s: State machine wait is done", d.Alias()),
				t, func() error {
					return runStep(d, m)
				}, nil,
			)
		}
		return nil
	}
	for d, m := range sms {
		log("===================================")
		log("%s: Starting machine", d.Alias())
		errs = append(errs, runStep(d, m))
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	for {
		log("Waiting for next process event: %s", time.Until(eq.Next()))
		ev := eq.Wait()
		if ev == nil {
			return fmt.Errorf("No more events in the queue")
		}
		// Wait just a little longer
		time.Sleep(5 * time.Second)

		log("===================================")
		log("%s: Processing machine", ev.Name)
		if err := ev.Callback(); err != nil {
			log("Processing error: %v", err)
		}
	}
}
