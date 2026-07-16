package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var travelCmd = &cobra.Command{
	Use:   "travel",
	Short: "Instruct a replicant to relocate",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		if len(args) == 0 || args[0] == "" {
			return fmt.Errorf("A destination is required")
		}
		dryRun, _ := cmd.Flags().GetBool("dry_run")
		via, _ := cmd.Flags().GetStringSlice("via")
		autoVia, _ := cmd.Flags().GetBool("auto_via")
		dest := args[0]
		res, err := rest.ReplicantTravel(rID, dest, via, dryRun)
		if err != nil {
			return fmt.Errorf("Error starting trip: %v", err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(res)
		} else {
			printTable([]string{
				"Origin", "Destination", "Status",
				"Duration", "Departed", "Arrives",
			}, [][]string{{
				string(res.Origin), string(res.Destination), res.Status,
				res.TotalTime.String(), t(res.Departed.Time()), t(res.Arrives.Time()),
			}})
			var ls [][]string
			for _, l := range res.Route {
				ls = append(ls, []string{
					d(l.Leg), string(l.From), string(l.To), l.Type, l.Time.String(),
				})
			}
			printTable([]string{"Leg", "From", "To", "Type", "Duration"}, ls)
		}
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop whatever you're doing",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		rep, err := rest.Replicant(rID)
		if err != nil {
			return err
		}
		v := rep.HostedDeviceCode
		if v == nil {
			return fmt.Errorf("Can't find vessel for %s", rep.Code.Alias())
		}
		res, err := rest.DeviceCommand[models.CommandResp](v, "deactivate", nil)
		if err != nil {
			return err
		}
		prettyPrint(res)

		return nil
	},
}

var teleportCmd = &cobra.Command{
	Use:   "teleport",
	Short: "Teleport to an empty matrix",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		target, _ := cmd.Flags().GetString("target")
		res, err := rest.ReplicantTeleport(rID, models.NewCodeAlias(target))
		if err != nil {
			return err
		}
		printTable([]string{
			"Replicant", "Status", "Source", "Destination", "Matrix", "Completes", "Online",
		}, [][]string{{
			rID.Alias(), res.Status, res.SourceStar, res.DestinationStar, res.TargetMatrixCode.Alias(),
			t(res.Completes.Time()), t(res.Completes.Time().Add(res.Offline.Duration())),
		}})
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(travelCmd)
	travelCmd.Flags().BoolP("dry_run", "n", false, "Only preview the route")
	travelCmd.Flags().StringSliceP("via", "v", []string{}, "Specify an explicit route")
	replicantCmd.AddCommand(stopCmd)

	replicantCmd.AddCommand(teleportCmd)
	teleportCmd.Flags().StringP("target", "t", "", "Matrix id to teleport to")
	teleportCmd.MarkFlagRequired("target")
}
