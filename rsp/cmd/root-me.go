package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var accountCmd = &cobra.Command{
	Use:     "account",
	Short:   "Show current status",
	Aliases: []string{"me"},
	RunE: func(cmd *cobra.Command, args []string) error {
		emailUpt := make(map[string]bool)
		webUpt := make(map[string]bool)
		emails := getStringSlice(cmd, "email")
		for _, e := range emails {
			if !strings.Contains(e, ":") {
				return fmt.Errorf("Invalid setting %q. Pass type:(on|off)", e)
			}
			bits := strings.Split(e, ":")
			emailUpt[bits[0]] = bits[1] == "on"
		}
		webs := getStringSlice(cmd, "webhook")
		for _, e := range webs {
			if !strings.Contains(e, ":") {
				return fmt.Errorf("Invalid setting %q. Pass type:(on|off)", e)
			}
			bits := strings.Split(e, ":")
			webUpt[bits[0]] = bits[1] == "on"
		}
		if coop := getString(cmd, "cooperation"); coop != "" {
			data := &models.AccountUpdate{
				ReplicantCooperation: coop,
			}
			res, err := rest.PatchSettings(data)
			if err != nil {
				return err
			}
			log(res.Status)
		}
		if len(emailUpt) > 0 || len(webUpt) > 0 {
			data := &models.AccountUpdate{
				MessageNotify: &models.Notify{
					Email:   true,
					Webhook: true,
					Preferences: &models.NotifyDetails{
						Email:   emailUpt,
						Webhook: webUpt,
					},
				},
			}

			res, err := rest.PatchSettings(data)
			if err != nil {
				return err
			}
			log(res.Status)
		}

		acc, err := rest.Account()
		if err != nil {
			return fmt.Errorf("Error getting status: %v", err)
		}
		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(acc)
			return nil
		}

		printTable(
			[]string{"Name", "Bobnet", "XP", "Status", "Unread messages", "Cooperation"},
			[][]string{{
				acc.Name,
				list(acc.BobnetChannels),
				d(acc.ExperiencePointsTotal),
				acc.Status,
				d(acc.UnreadMessageCount),
				acc.ReplicantCooperation,
			}})

		var mn [][]string
		mn = append(mn, []string{
			"Enabled",
			b(acc.MessageNotify.Email),
			b(acc.MessageNotify.Webhook),
		})
		var types []string
		for k := range acc.MessageNotify.Preferences.Email {
			types = append(types, k)
		}
		slices.Sort(types)
		for _, t := range types {
			mn = append(mn, []string{
				strings.ToUpper(t[0:1]) + t[1:],
				b(acc.MessageNotify.Preferences.Email[t]),
				b(acc.MessageNotify.Preferences.Webhook[t]),
			})
		}
		printTable([]string{"Type", "Email", "Webhook"}, mn)

		var reps [][]string
		var names []string
		for name := range acc.Replicants {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			r := acc.Replicants[name]
			code, err := db.Alias(r.Code.String(), "replicant")
			if err != nil {
				return fmt.Errorf("Error creating alias for %q: %v", err)
			}

			reps = append(reps, []string{
				r.Name,
				code,
				string(r.CurrentLocation),
				d(r.ExperiencePoints),
				r.Status,
			})
		}
		printTable([]string{"Name", "Code", "Location", "XP", "Status"}, reps)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(accountCmd)
	accountCmd.Flags().StringSliceP("email", "e", nil, "Adjust email notification: type:(on|off)")
	accountCmd.Flags().StringSliceP("webhook", "w", nil, "Adjust webhook notification: type:(on|off)")
	accountCmd.Flags().String("cooperation", "", "Adjust account cooperation mode: (individual|shared)")
}
