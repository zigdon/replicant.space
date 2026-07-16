package cmd

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"

	lg "charm.land/lipgloss/v2"
)

func eventComplete(eid string) error {
	e, err := rest.CompleteEvent(eid)
	if err != nil {
		return err
	}
	var xp, civ int
	var rs []string
	if e.Rewards != nil {
		xp = e.Rewards.XP
		civ = e.Rewards.CivilisationPoints
		for k, v := range e.Rewards.Resources {
			rs = append(rs, fmt.Sprintf("%d x %s", v, k))
		}
	}
	printTable([]string{
		"Designation", "Title", "Status", "XP", "Civ Points", "Resources",
	}, [][]string{{
		e.Designation, e.Title, e.Status, d(xp), d(civ), lines(rs),
	}})
	return nil
}

var eventCompleteCmd = &cobra.Command{
	Use:   "complete",
	Short: "Trigger event completion",
	RunE: func(cmd *cobra.Command, args []string) error {
		eventID, _ := cmd.Flags().GetString("id")
		return eventComplete(eventID)
	},
}

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "See all your current ongoing events",
	RunE: func(cmd *cobra.Command, args []string) error {
		noDetails, _ := cmd.Flags().GetBool("list")
		eventID, _ := cmd.Flags().GetString("id")
		width, _ := cmd.Flags().GetInt("width")
		style := lg.NewStyle().Width(width)
		data, err := rest.Events()
		if err != nil {
			return fmt.Errorf("Error getting events: %v", err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(data)
			return nil
		}
		if noDetails {
			var events [][]string
			for _, e := range data.Events {
				events = append(events, []string{
					e.Title, e.Designation, e.Location, e.Category, e.Status, d(e.Tier),
				})
			}
			printTable([]string{
				"Title", "Designation", "Location", "Category", "Status", "Tier",
			}, events)
			return nil
		}

		for _, e := range data.Events {
			if eventID != "" && e.Designation != eventID {
				continue
			}
			printEvent(e, style)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(eventsCmd)
	eventsCmd.Flags().IntP("width", "w", 50, "Wrap message body to this width")
	eventsCmd.Flags().String("id", "", "Show only this event")
	eventsCmd.Flags().BoolP("list", "l", false, "Show only event list, no details")

	eventsCmd.AddCommand(eventCompleteCmd)
	eventCompleteCmd.Flags().String("id", "", "Show only this event")
	eventCompleteCmd.MarkFlagRequired("id")
}

func printEvent(e *models.Event, style lg.Style) {
	fmt.Println(strings.Repeat("=", 60))
	printTable([]string{
		"Title", "Type", "Designation", "Location", "Category", "Discovered", "Status", "Tier",
	}, [][]string{{
		e.Title, e.Type, e.Designation, e.Location, e.Category, e.Discovered.String(), e.Status, d(e.Tier),
	}})
	printTable([]string{
		"Rewards: XP", "Civ Points", "Achievement", "Resources",
	}, [][]string{{
		d(e.Rewards.XP),
		d(e.Rewards.CivilisationPoints),
		e.Rewards.CompletionAchievement,
		m(e.Rewards.Resources),
	}})
	printTable([]string{}, [][]string{
		{style.Render(e.Description + "\n")},
		{style.Render(e.BroadcastMessage)}})
	var crit [][]string
	for _, c := range e.Criteria {
		crit = append(crit, []string{
			c.Name, formatDev(c.Devices, true), m(c.Resources),
		})
	}
	printTable([]string{"Criteria", "Devices", "Resources"}, crit)

	var progress [][]string
	for _, p := range e.Progress.Options {
		line := []string{p.Name, b(p.Met), formatDev(p.Devices, false)}
		var delivered []string
		for _, r := range p.Resources {
			var st string
			if r.Met {
				st = "✅"
			} else {
				st = fmt.Sprintf("%.2f/%d", r.Current, r.Required)
			}
			delivered = append(delivered, fmt.Sprintf("%s: %s", r.ResourceType, st))
		}
		line = append(line, lines(delivered))
		progress = append(progress, line)
	}
	printTable([]string{"Name", "Done", "Devices", "Resources"}, progress)
}

func formatDev(devs []*models.EventDevice, resBreakdown bool) string {
	bps := make(map[string]*models.Blueprint)
	var bpRes func(string) (map[string]int, map[string]int)
	var errs []error
	bpRes = func(dt string) (map[string]int, map[string]int) {
		bp, ok := bps[dt]
		res := make(map[string]int)
		dev := make(map[string]int)
		if !ok {
			bp = &models.Blueprint{DeviceType: dt}
			if err := bp.Get(); err != nil {
				errs = append(errs, fmt.Errorf("Can't load blueprint %q: %v", dt, err))
				dev[dt] = 1
				return nil, dev
			}
			bps[dt] = bp
			for r, q := range bp.Resources {
				res[r] += q
			}
			for r, q := range bp.Components {
				dev[r] = q
				subres, subdev := bpRes(r)
				for k, v := range subres {
					res[k] += v * q
				}
				for k, v := range subdev {
					dev[k] += v * q
				}
			}
		}
		return res, dev
	}
	var out []string
	res := make(map[string]int)
	dev := make(map[string]int)
	for _, d := range devs {
		dt := d.DeviceType
		out = append(out, fmt.Sprintf("%d x %s", d.Required, dt))
		if !resBreakdown {
			continue
		}
		subres, subdev := bpRes(dt)
		for r, q := range subres {
			res[r] += q * d.Count
		}
		for r, q := range subdev {
			dev[r] += q * d.Count
		}
	}
	var rs []string
	for k := range res {
		rs = append(rs, k)
	}
	slices.Sort(rs)
	for _, r := range rs {
		if res[r] == 0 {
			continue
		}
		out = append(out, fmt.Sprintf("(%4d x %s)", res[r], r))
	}
	out = append(out, "")
	rs = []string{}
	for k := range dev {
		rs = append(rs, k)
	}
	if len(rs) > 0 {
		slices.Sort(rs)
		for _, r := range rs {
			if dev[r] == 0 {
				continue
			}
			out = append(out, fmt.Sprintf("(%4d x %s)", dev[r], r))
		}
		out = append(out, "")
	}

	if err := errors.Join(errs...); err != nil {
		log("Errors:\n%v", err)
	}

	return strings.Join(out, "\n")
}
