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
		dryRun := getBool(cmd, "dry_run")
		via := getStringSlice(cmd, "via")
		dest := args[0]
		res, err := rest.ReplicantTravel(rID, dest, via, dryRun)
		if err != nil {
			return fmt.Errorf("Error starting trip: %v", err)
		}
		if raw := getBool(cmd, "raw"); raw {
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

func getTeleportDests(loc string) ([]*models.Device, error) {
	var cradles []*models.Device
	rows, err := db.DB.Query(
		`SELECT blueprint_type FROM blueprint_features WHERE feature = 'cradle';`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		log("Searching for %s...", t)
		cfg := map[string]string{
			"device_type": t,
		}
		if loc != "" {
			cfg["location"] = loc
		}
		devs, err := rest.RefreshDevices(cfg)
		if err != nil {
			return nil, err
		}
		if len(devs) > 0 {
			log("... %v", devList(devs))
			cradles = append(cradles, devs...)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Now check each cradle, and see which have an empty matrix
	var res []*models.Device
	for _, c := range cradles {
		c, err := rest.RefreshDeviceInfo(c.Code)
		if err != nil {
			return nil, err
		}
		if c.StowedDevices == nil {
			log("%s: Nothing stowed", c.Code.Alias())
			continue
		}
		if !slices.ContainsFunc(c.StowedDevices.Devices, func(d *models.StowedDevice) bool {
			if d.Type == "empty_replicant_matrix" {
				return true
			}
			return false
		}) {
			continue
		}
		res = append(res, c)
	}

	return res, nil
}

var teleportCmd = &cobra.Command{
	Use:   "teleport",
	Short: "Teleport to an empty matrix",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		target := getString(cmd, "target")
		if target == "" {
			loc := getString(cmd, "location")
			dests, err := getTeleportDests(loc)
			if err != nil {
				return err
			}
			for _, d := range dests {
				log("%s @ %s...", d.Code.Alias(), d.Location)
				if string(d.Location) == loc {
					log("...bullseye")
					target = d.StowedDevices.Devices[0].Code.Alias()
					break
				}
				if strings.HasPrefix(loc, d.Location.Star()) {
					log("...in system")
					target = d.StowedDevices.Devices[0].Code.Alias()
				}
			}
			if target == "" {
				return fmt.Errorf("No empty matrixes found at %s", loc)
			}
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

var teleportListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all teleport destinations",
	RunE: func(cmd *cobra.Command, args []string) error {
		dests, err := getTeleportDests("")
		if err != nil {
			return err
		}
		var data [][]string
		for _, d := range dests {
			data = append(data, []string{
				d.Code.Alias(),
				string(d.Location),
				d.Type,
				d.StowedDevices.Devices[0].Code.Alias(),
			})
		}
		printTable([]string{"Code", "Location", "Type", "Matrix"}, data)
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
	teleportCmd.Flags().StringP("location", "l", "", "Location to teleport to")
	teleportCmd.MarkFlagsOneRequired("target", "location")

	teleportCmd.AddCommand(teleportListCmd)
}
