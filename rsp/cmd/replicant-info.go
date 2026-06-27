package cmd

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var replicantInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get replicant details",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		repl, err := rest.Replicant(rID)
		if err != nil {
			return fmt.Errorf("Error getting replicant: %v", err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			fmt.Printf("%T\n", repl)
			prettyPrint(repl)
			return nil
		}
		printTable([]string{
			"Name", "Code", "Location", "XP", "Description", "Status",
		}, [][]string{{
			repl.Name, repl.Code.Alias(), repl.Location,
			d(repl.ExperiencePoints), repl.Description, repl.Status,
		}})
		if repl.Travel != nil {
			trip := repl.Travel
			printTable([]string{
				"Departed", "Arrives", "ETA", "Stage",
			}, [][]string{{
				trip.Origin, trip.Destination, trip.Eta.String(), trip.Stage,
			}, {
				trip.Departed.String(), trip.Arrives.String(), p(trip.ProgressPercent), "",
			}})
			var legs [][]string
			for _, l := range trip.Route {
				dist := l.DistanceAu + l.DistanceLy
				legs = append(legs, []string{
					d(l.Leg), b(l.Active), l.From, l.To, l.Time.String(), f(dist), l.Type,
				})
			}
			printTable([]string{"Leg", "Active", "From", "To", "Time", "Distance", "Type"}, legs)
		}
		if len(repl.StowedDevices) > 0 {
			cnt := make(map[string]int)
			for _, d := range repl.StowedDevices {
				cnt[d.Type]++
			}
			var types [][]string
			for t, n := range cnt {
				types = append(types, []string{fmt.Sprintf("%d", n), t})
			}
			slices.SortFunc(types, func(a, b []string) int {
				return cmp.Compare(a[1], b[1])
			})
			printTable([]string{"Count", "Stowed"}, types)
		}
		if repl.Printing != nil {
			pr := repl.Printing
			printTable([]string{"Printing", "Started", "Completes", "ETA", "Progress"},
				[][]string{{pr.DeviceType, t(pr.Started.Time()), t(pr.Completes.Time()),
					pr.Eta.String(), p(pr.ProgressPercent)}})
		}
		if len(repl.PrintQueue) > 0 {
			var q [][]string
			for _, pq := range repl.PrintQueue {
				q = append(q, []string{
					pq.DeviceType,
					pq.Notify.Device,
					b(pq.Notify.Email),
					b(pq.Notify.Webhook),
				})
			}
			printTable([]string{
				"Type", "Notify device", "Notify email", "Notify webhook",
			}, q)
		}
		if len(repl.WaitingFor) > 0 {
			var w [][]string
			for k, v := range repl.WaitingFor {
				w = append(w, []string{
					k, d(v.Have), d(v.Need),
				})
			}
			printTable([]string{"Resource", "Have", "Need"}, w)
		}
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(replicantInfoCmd)
}
