package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"

	lg "charm.land/lipgloss/v2"
)

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
		} else if noDetails {
			var events [][]string
			for _, e := range data.Events {
				events = append(events, []string{
					e.Title, e.Type, e.Designation, e.Location, e.Category, e.DiscoveredAt, e.Status, d(e.Tier),
				})
			}
			printTable([]string{
				"Title", "Type", "Designation", "Location", "Category", "Discovered", "Status", "Tier",
			}, events)
		} else {
			for _, e := range data.Events {
				if eventID != "" && e.Designation != eventID { continue }
				printTable([]string{
					"Title", "Type", "Designation", "Location", "Category", "Discovered", "Status", "Tier",
				}, [][]string{{
					e.Title, e.Type, e.Designation, e.Location, e.Category, e.DiscoveredAt, e.Status, d(e.Tier),
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
					{style.Render(e.Description+"\n")},
				    {style.Render(e.BroadcastMessage),
				}})
				var crit [][]string
				for _, c := range e.Criteria {
					crit = append(crit, []string{
						c.Name, v(c.Devices), m(c.Resources),
					})
				}
				printTable([]string{"Criteria", "Devices", "Resources"}, crit)
				
				var progress [][]string
				for _, p := range e.Progress.Options {
					line := []string{p.Name, b(p.Met), v(p.Devices)}
					var delivered []string
					for _, r := range p.Resources {
						var st string
						if r.Met {
							st = "✅"
						} else {
							st = fmt.Sprintf("%d/%d", r.Current, r.Required)
						}
						delivered = append(delivered, fmt.Sprintf("%s: %s", r.ResourceType, st))
					}
					line = append(line, lines(delivered))
					progress = append(progress, line)
				}
				printTable([]string{"Name", "Done", "Devices", "Resources"}, progress)

			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(eventsCmd)
	eventsCmd.Flags().IntP("width", "w", 50, "Wrap message body to this width")
	eventsCmd.Flags().String("id", "", "Show only this event")
	eventsCmd.Flags().BoolP("list", "l", false, "Show only event list, no details")
}
