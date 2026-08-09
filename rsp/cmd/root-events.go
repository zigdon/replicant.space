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

const home = "MENKUNT-2-L4"

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
			rs = append(rs, fmt.Sprintf("%d × %s", v, k))
		}
	}
	printTable([]string{
		"Designation", "Title", "Status", "XP", "Civ Points", "Resources",
	}, [][]any{{
		e.Designation, e.Title, e.Status, xp, civ, lines(rs),
	}})
	return nil
}

var eventCompleteCmd = &cobra.Command{
	Use:   "complete",
	Short: "Trigger event completion",
	RunE: func(cmd *cobra.Command, args []string) error {
		eventID := getString(cmd, "id")
		return eventComplete(eventID)
	},
}

var eventsCmd = &cobra.Command{
	Use:     "events",
	Aliases: []string{"event"},
	Short:   "See all your current ongoing events",
	RunE: func(cmd *cobra.Command, args []string) error {
		noDetails := getBool(cmd, "list")
		eventID := getString(cmd, "id")
		width := getInt(cmd, "width")
		style := lg.NewStyle().Width(width)
		data, err := rest.Events()
		if err != nil {
			return fmt.Errorf("Error getting events: %v", err)
		}
		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(data)
			return nil
		}
		if noDetails {
			printEventSummary(data.Events)
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

var megaContributeCmd = &cobra.Command{
	Use:   "contribute",
	Short: "Contribute to a megastructure",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := getString(cmd, "id")
		ds := getStringSlice(cmd, "device")
		raw := getBool(cmd, "raw")
		var devs []*models.CodeAlias
		for _, d := range ds {
			ids := explode(d, models.LocationID(id))
			for _, id := range ids {
				devs = append(devs, models.NewCodeAlias(id))
			}
		}
		if len(devs) == 0 {
			return fmt.Errorf("No devices specified")
		}
		res, err := rest.Contribute(id, devs)
		if err != nil {
			return err
		}
		if raw {
			prettyPrint(res)
			return nil
		}
		if res.Error == "" {
			printTable([]string{"Location", "Status", "Leaderboard", "Contributions",
				"Value", "Progress", "Stage"}, [][]any{{
				res.Location, res.Status, res.LeaderboardPosition,
				res.YourTotalContributions, res.YourTotalValue,
				p(res.Progress.Percentage), res.Progress.Stage,
			}})
		}
		accepted := make(map[string][]string)
		values := make(map[string]int)
		types := make(map[string]bool)
		for _, d := range res.Accepted {
			id := d.DeviceCode
			accepted[id.Type()] = append(accepted[id.Type()], id.Alias())
			values[id.Type()] += d.Value
			types[id.Type()] = true
		}
		var tList []string
		for k := range types {
			tList = append(tList, k)
		}

		slices.Sort(tList)
		var data [][]any
		for _, t := range tList {
			data = append(data, []any{
				t, values[t], len(accepted[t]), list(accepted[t]),
			})
		}

		if len(data) > 0 {
			printTable([]string{"Type", "Value", "Count", "Accepted"}, data)
		}

		rejected := make(map[string][]string)
		for _, d := range res.Rejected {
			rejected[d.Reason] = append(rejected[d.Reason], d.DeviceCode.Alias())
		}
		data = [][]any{}
		for k, v := range rejected {
			data = append(data, []any{k, list(v)})
		}
		if len(data) > 0 {
			printTable([]string{"Reason", "Rejected"}, data)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(eventsCmd)
	eventsCmd.Flags().IntP("width", "w", 50, "Wrap message body to this width")
	eventsCmd.Flags().String("id", "", "Show only this event")
	eventsCmd.Flags().BoolP("list", "l", false, "Show only event list, no details")
	eventsCmd.RegisterFlagCompletionFunc("id", completeEventIDs)

	eventsCmd.AddCommand(eventCompleteCmd)
	eventCompleteCmd.Flags().String("id", "", "Show only this event")
	eventCompleteCmd.MarkFlagRequired("id")
	eventCompleteCmd.RegisterFlagCompletionFunc("id", completeEventIDs)
	eventCompleteCmd.RegisterFlagCompletionFunc("criteria", completeEventCriteria)

	rootCmd.AddCommand(megaContributeCmd)
	megaContributeCmd.Flags().String("id", "", "Megastructure ID")
	megaContributeCmd.Flags().StringSliceP("device", "d", []string{}, "Devices to contribute")
	megaContributeCmd.MarkFlagRequired("id")
	megaContributeCmd.MarkFlagRequired("device")
}

func printEventSummary(es []*models.Event) {
	var events [][]any
	for _, e := range es {
		tag := fmt.Sprintf("event:%s", e.Designation)
		txTag := fmt.Sprintf("tx:%s", e.Designation)
		var from, transit, to int
		devs, dErr := rest.Devices(map[string]string{"tag": tag})
		tx, txErr := rest.Devices(map[string]string{"tag": txTag})
		if dErr != nil || txErr != nil {
			log("Error getting devices for %q: %v, %v", tag, dErr, txErr)
		} else {
			devs = append(devs, tx...)
			for _, d := range devs {
				switch d.Location {
				case home:
					from++
				case e.Location:
					to++
				case "":
					transit++
				default:
					log("%s is off-track, currently at %q", d, d.Location)
				}
			}
		}
		events = append(events, []any{
			e.Title, e.Designation, e.Location, e.Category, e.Status, e.Tier, from, transit, to,
		})
	}
	printTable([]string{
		"Title", "Designation", "Location", "Category", "Status", "Tier", "Home", "Transit", "Delivered",
	}, events)
}

func printEvent(e *models.Event, style lg.Style) {
	fmt.Println(strings.Repeat("=", 60))
	printTable([]string{
		"Title", "Type", "Designation", "Location", "Category", "Discovered", "Status", "Tier",
	}, [][]any{{
		e.Title, e.Type, e.Designation, e.Location, e.Category, e.Discovered, e.Status, e.Tier,
	}})
	printTable([]string{
		"Rewards: XP", "Civ Points", "Achievement", "Resources",
	}, [][]any{{
		e.Rewards.XP,
		e.Rewards.CivilisationPoints,
		e.Rewards.CompletionAchievement,
		m(e.Rewards.Resources),
	}})
	printTable([]string{}, [][]any{
		{style.Render(e.Description + "\n")},
		{style.Render(e.BroadcastMessage)}})
	var crit [][]any
	inv, err := rest.Devices(map[string]string{"location": home})
	if err != nil {
		log("Error getting home inventory: %v", err)
	}
	tag := fmt.Sprintf("event:%s", strings.ToLower(e.Designation))
	tagged, err := rest.Devices(map[string]string{"tag": tag})
	if err != nil {
		log("Error getting tagged devices: %v", err)
	}
	txTag := fmt.Sprintf("tx:%s", strings.ToLower(e.Designation))
	tx, err := rest.Devices(map[string]string{"tag": txTag})
	if err != nil {
		log("Error getting tagged transports: %v", err)
	}
	tagged = append(tagged, tx...)
	for _, c := range e.Criteria {
		crit = append(crit, []any{
			c.Name, formatDev(c.Devices, true),
			ready(c.Devices, tagged),
			unassigned(tag, c.Devices, inv),
			m(c.Resources),
		})
	}
	printTable([]string{"Criteria", "Devices", "Ready", "Unassigned", "Resources"}, crit)

	var progress [][]any
	for _, p := range e.Progress.Options {
		line := []any{p.Name, p.Met, formatDev(p.Devices, false)}
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

	var data [][]any
	for _, d := range tagged {
		loc := d.Location
		if loc == "" {
			loc = "in transit"
		}
		data = append(data, []any{d.Code.Alias(), d.Type, loc, list(d.Tags)})
	}
	printTable([]string{"Device", "Type", "Location", "Tags"}, data)

}

func ready(devs []*models.EventDevice, inv []*models.Device) string {
	need := make(map[string]bool)
	for _, d := range devs {
		need[d.DeviceType] = true
	}
	var res []string
	for _, d := range inv {
		if !need[d.Type] {
			continue
		}
		if d.StowedDevices != nil || len(d.AttachedDevices) > 0 {
			continue
		}
		loc := string(d.Location)
		if loc == "" && d.AttachedToDeviceCode != nil {
			loc = d.AttachedToDeviceCode.Alias()
		} else if loc == "" {
			loc = "In transit?"
		}
		res = append(res, fmt.Sprintf("%s (%s) @ %s", d.Type, d.Code.Alias(), loc))
	}

	return strings.Join(res, "\n")
}

func unassigned(tag string, devs []*models.EventDevice, inv []*models.Device) string {
	homeDevs := make(map[string][]*models.CodeAlias)
	for _, d := range inv {
		if slices.Contains(d.Tags, tag) {
			continue
		}
		homeDevs[d.Type] = append(homeDevs[d.Type], d.Code)
	}
	var res []string
	for _, d := range devs {
		if ds, ok := homeDevs[d.DeviceType]; ok {
			if len(ds) == 1 {
				res = append(res, fmt.Sprintf("%s (%s)", d.DeviceType, ds[0].Alias()))
			} else {
				res = append(res, fmt.Sprintf("%s x %d (%s)", d.DeviceType, len(ds), ds[0].Alias()))
			}
		}
	}

	return strings.Join(res, "\n")
}

func formatDev(devs []*models.EventDevice, resBreakdown bool) string {
	bps := make(map[string]*models.Blueprint)
	var errs []error
	var bpRes func(string) (map[string]int, map[string]int)
	bpRes = func(dt string) (map[string]int, map[string]int) {
		res := make(map[string]int)
		dev := make(map[string]int)
		bp, ok := bps[dt]
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
		out = append(out, fmt.Sprintf("%d × %s", d.Required, dt))
		if !resBreakdown {
			continue
		}
		subres, subdev := bpRes(dt)
		for r, q := range subres {
			res[r] += q * d.Required
		}
		for r, q := range subdev {
			dev[r] += q * d.Required
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
