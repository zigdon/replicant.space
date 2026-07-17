package cmd

import (
	"fmt"
	"slices"
	"strings"

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
		if len(via) == 0 && autoVia {
			_, err := models.NewStar(dest)
			if err != nil {
				return err
			}
			// hub, err := db.FindNearestHub
		}
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
		if target == "" {
			loc, _ := cmd.Flags().GetString("location")
			var cradles []*models.Device
			rows, err := db.DB.Query(
				`SELECT blueprint_type FROM blueprint_features WHERE feature = 'cradle';`)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var t string
				if err := rows.Scan(&t); err != nil {
					return err
				}
				devs, err := rest.Devices(map[string]string{"device_type": t})
				if err != nil {
					return err
				}
				cradles = append(cradles, devs...)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			// Now check each cradle, and see which have an empty matrix
			var erms []*models.CodeAlias
			for _, c := range cradles {
				var erm *models.CodeAlias
				if !slices.ContainsFunc(c.StowedDevices.Devices, func(d *models.StowedDevice) bool {
					if d.Type == "empty_replicant_matrix" {
						erm = d.Code
						return true
					}
					return false
				}) {
					continue
				}
				if string(c.Location) == loc {
					erms = []*models.CodeAlias{erm}
					break
				}
				if strings.HasPrefix(loc, c.Location.Star()) {
					erms = append(erms, erm)
				}
			}
			if len(erms) == 0 {
				return fmt.Errorf("No empty matrixes found at %s", loc)
			}
			target = erms[0].String()
		}
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
	travelCmd.Flags().Bool("auto_via", false, "Automatically use a hub waypoint when possible")
	replicantCmd.AddCommand(stopCmd)

	replicantCmd.AddCommand(teleportCmd)
	teleportCmd.Flags().StringP("target", "t", "", "Matrix id to teleport to")
	teleportCmd.Flags().StringP("location", "l", "", "Location to teleport to")
	teleportCmd.MarkFlagsOneRequired("target", "location")
}
