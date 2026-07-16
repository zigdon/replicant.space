package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// Look up the requirements for an event
// If there are multiple options
//   check that we have all the required blueprints
//   require the user specify which one to take
// Check which of the requirements are still missing
// If there are missing components
//   check if we already have some available (with either no tag or event:id tag)
//   if not, check if they're already being printed with an 'event:id' tag
//   if not, queue them
//   ship them
// If there are missing resources
//   check if there are already any in transit
//   if not, ship them
// Once everything is in place
//   if there's a replicant there, resolve the event
//   if not, check if there's an ERM, and teleport to it
//   if not, send the nearest replicant there

func autoEvent(cmd *cobra.Command, args []string) error {
	// Load event details
	eID, _ := cmd.Flags().GetString("event_id")
	evs, err := rest.Events()
	if err != nil {
		return err
	}
	var ev *models.Event
	var data [][]string
	for _, e := range evs.Events {
		data = append(data, []string{
			e.Designation, e.Title, e.Location,
		})
		if e.Designation != eID {
			continue
		}
		ev = e
	}
	if ev == nil {
		var eventsDesc *strings.Builder
		printTablef(eventsDesc, []string{"ID", "Title", "Location"}, data)
		return fmt.Errorf("Can't find event ID %q. Pick from:\n%s", eID, eventsDesc.String())
	}

	// Load the blueprints we know
	bps := make(map[string]bool)
	blueprints, err := db.ListIDs(cache.BlueprintsTable)
	if err != nil {
		return err
	}
	for _, bp := range blueprints {
		bps[bp.(string)] = true
	}

	// Examine resolution options
	var ecs []*models.EventCriteria
	data = [][]string{}
	for n, cr := range ev.Criteria {
		canDo := true
		for _, bp := range cr.Devices {
			if !bps[bp.DeviceType] {
				log("Missing blueprint %s for %s", bp.DeviceType, cr.Name)
				canDo = false
				break
			}
		}
		if !canDo {
			continue
		}
		data = append(data, []string{
			d(n + 1), v(cr.Resources), v(cr.Devices),
		})
		ecs = append(ecs, cr)
	}

	// If we have more than one possible way to go about it, make the user pick
	if len(ecs) == 0 {
		return fmt.Errorf("No valid paths available")
	}
	var ec *models.EventCriteria
	if len(ecs) > 1 {
		cid, _ := cmd.Flags().GetInt("criteria")
		if cid == 0 {
			var paths *strings.Builder
			printTablef(paths, []string{"ID", "Resources", "Devices"}, data)
			return fmt.Errorf("Multiple paths available, select one:\n%s", paths)
		}
		ec = ecs[cid-1]
	} else {
		ec = ecs[0]
	}
	prettyPrint(ec)

	return nil
}
