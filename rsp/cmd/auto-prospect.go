package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/auto"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

func autoProspect(cmd *cobra.Command, args []string) error {
	// If a device was specified, process only that one. Otherwise, loop over
	// all the observatories.
	var devs []*models.Device
	devIDs, _ := cmd.Flags().GetStringSlice("device")
	if len(devIDs) == 0 {
		var err error
		devs, err = rest.Devices(map[string]string{"device_type": "galactic_observatory"})
		if err != nil {
			return err
		}
	} else {
		for _, d := range devIDs {
			i, err := getInfo(models.NewCodeAlias(d))
			if err != nil {
				return err
			}
			devs = append(devs, i)
		}
	}
	if len(devs) == 0 {
		return fmt.Errorf("No observatories found")
	}

	var sms = make(map[*models.CodeAlias]auto.Machine)
	dryRun, _ := cmd.Flags().GetBool("dry_run")
	for _, d := range devs {
		sms[d.Code] = &auto.ProspectMachine{}
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
		log("Waiting for next process event: %s", time.Until(eq.Next()))
		eq.Wait()
	}
}
