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
	var sms = make(map[string]auto.Machine)
	dryRun := getBool(cmd, "dry_run")
	eq := auto.NewEventQueue(5 * time.Minute)
	var errs []error
	var runStep func(d *models.CodeAlias, m auto.Machine) error
	findSMs := func() error {
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

		for _, d := range devs {
			alias := d.Code.Alias()
			if _, ok := sms[alias]; ok {
				continue
			}
			log("** New machine: %s (%v)", d, d.Tags)
			if slices.Contains(d.Tags, "auto:prospect") {
				log("%s: prospect -> %v", d.Code.Alias(), d.Tags)
				sms[alias] = &auto.ProspectMachine{}
			} else if slices.Contains(d.Tags, "auto:relay") {
				log("%s: relay -> %v", d.Code.Alias(), d.Tags)
				sms[alias] = &auto.RelayMachine{}
			} else if slices.Contains(d.Tags, "auto:divert") {
				log("%s: divert -> %v", d.Code.Alias(), d.Tags)
				sms[alias] = &auto.DivertMachine{}
			} else {
				errs = append(errs, fmt.Errorf("Unknown state machine for %q: %v", d.Code.Alias(), d.Tags))
			}
			log("===================================")
			log("%s: Starting machine", d.Code.Alias())
			if err := sms[alias].Start(d, dryRun); err != nil {
				errs = append(errs, fmt.Errorf("Removing state machine %q: %v", d.Code, err))
				delete(sms, alias)
				continue
			}
			errs = append(errs, runStep(d.Code, sms[alias]))
		}
		return errors.Join(errs...)
	}
	runStep = func(d *models.CodeAlias, m auto.Machine) error {
		t, err := m.Process()
		if err != nil {
			if _, ok := errors.AsType[auto.MachineDoneErr](err); ok {
				log("State machine %s (%s) done: %v", d, m.Name(), err)
				delete(sms, d.Alias())
				return nil
			}
			errs = append(errs, err)
		} else if t.IsZero() {
			errs = append(errs, fmt.Errorf("%s: No time for next step", d.Alias()))
		} else {
			eq.AddEvent(
				d.Alias(),
				fmt.Sprintf("%s: State machine %s wait is done", d.Alias(), m.Name()),
				t, func() error {
					return runStep(d, m)
				}, nil,
			)
		}
		return nil
	}
	for {
		if err := findSMs(); err != nil {
			log("Error finding new state machines: %v", err)
		}
		evs := eq.List()
		var data [][]any
		for _, e := range evs {
			data = append(data, []any{e.When.Format(time.Stamp), e.Name, e.Desc})
		}
		printTable([]string{"When", "Who", "What"}, data)
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
		if len(args) > 0 {
			log("Stopping after one iteration")
			break
		}
	}
	return nil
}
