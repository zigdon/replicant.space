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
		devs = append(devs, res.Devices...)
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
			sms[d.Code] = &auto.ProspectMachine{}
		} else if slices.Contains(d.Tags, "auto:relay") {
			sms[d.Code] = &auto.RelayMachine{}
		} else {
			return fmt.Errorf("Unknown state machine for %q: %v", d.Code.Alias(), d.Tags)
		}
		if err := sms[d.Code].Start(d, dryRun); err != nil {
			return err
		}
	}

	eq := auto.NewEventQueue(5 * time.Minute)
	for {
		var errs []error
		for d, m := range sms {
			log("%s: Processing machine", d.Alias())
			t, err := m.Process()
			if err != nil {
				errs = append(errs, err)
			} else if t.IsZero() {
				errs = append(errs, fmt.Errorf("%s: No time for next step", d.Alias()))
			} else {
				eq.AddEvent(
					d.Alias(),
					fmt.Sprintf("%s: State machine wait is done", d.Alias()),
					t, nil,
				)
			}
		}
		if err := errors.Join(errs...); err != nil {
			log("Errors: %v", err)
		}
		if dryRun {
			break
		}
		log("Waiting for next process event: %s", time.Until(eq.Next()))
		eq.Wait()
	}
	return nil
}
