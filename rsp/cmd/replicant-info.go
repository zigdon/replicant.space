package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"time"

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
		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(repl)
			return nil
		}
		printTable([]string{
			"Name", "Code", "Location", "XP", "Description", "Status", "Vessel",
		}, [][]any{{
			repl.Name, repl.Code, repl.Location,
			repl.ExperiencePoints, repl.Description, repl.Status,
			repl.HostedDeviceCode,
		}})
		if repl.Travel != nil {
			trip := repl.Travel
			printTable([]string{
				"Departed", "Arrives", "ETA", "Stage",
			}, [][]any{{
				trip.Origin, trip.Destination, trip.Eta, trip.Stage,
			}, {
				trip.Departed.Time().Format(time.Kitchen),
				trip.Arrives.Time().Format(time.Kitchen),
				p(trip.ProgressPercent), "",
			}})
			var legs [][]any
			for _, l := range trip.Route {
				dist := l.DistanceAu + l.DistanceLy
				legs = append(legs, []any{
					l.Leg, l.Active, l.From, l.To, l.Time, dist, l.Type,
				})
			}
			printTable([]string{"Leg", "Active", "From", "To", "Time", "Distance", "Type"}, legs)
		}
		if len(repl.StowedDevices) > 0 {
			cnt := make(map[string][]string)
			for _, d := range repl.StowedDevices {
				cnt[d.Type] = append(cnt[d.Type], d.Code.Alias())
			}
			var types [][]any
			for t, n := range cnt {
				types = append(types, []any{fmt.Sprintf("%d", len(n)), t, list(n)})
			}
			slices.SortFunc(types, func(a, b []any) int {
				return cmp.Compare(a[1].(string), b[1].(string))
			})
			printTable([]string{"Count", "Stowed", "IDs"}, types)
		}
		if repl.Printing != nil {
			pr := repl.Printing
			printTable([]string{"Printing", "Started", "Completes", "ETA", "Progress"},
				[][]any{{pr.DeviceType, pr.Started.Time(), pr.Completes.Time(),
					pr.Eta, p(pr.ProgressPercent)}})
		}
		if len(repl.PrintQueue) > 0 {
			var q [][]any
			for _, pq := range repl.PrintQueue {
				q = append(q, []any{
					pq.DeviceType,
					pq.Notify.Device,
					pq.Notify.Email,
					pq.Notify.Webhook,
				})
			}
			printTable([]string{
				"Type", "Notify device", "Notify email", "Notify webhook",
			}, q)
		}
		if len(repl.WaitingFor.Components) > 0 || len(repl.WaitingFor.Resources) > 0 {
			var w [][]any
			for k, v := range repl.WaitingFor.Resources {
				w = append(w, []any{k, v.Have, v.Need})
			}
			for k, v := range repl.WaitingFor.Components {
				w = append(w, []any{k, v.Have, v.Need})
			}
			printTable([]string{"Resource", "Have", "Need"}, w)
		}
		if repl.Teleport != nil {
			rt := repl.Teleport
			printTable([]string{"Teleport status", "Source", "Destination", "Started", "Completes", "Target"},
				[][]any{{
					rt.Status, rt.SourceStar, rt.DestinationStar, rt.Started.Time(), rt.Completes.Time(),
					rt.TargetMatrixCode,
				}})
		}
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(replicantInfoCmd)
}
