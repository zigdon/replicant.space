package cmd

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var travelCmd = &cobra.Command{
	Use:               "travel",
	Short:             "Instruct a replicant to relocate",
	ValidArgsFunction: completeStarsAndPlanets,
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		if len(args) == 0 || args[0] == "" {
			return fmt.Errorf("A destination is required")
		}
		rep, err := rest.Replicant(rID)
		if err != nil {
			return err
		}
		dryRun := getBool(cmd, "dry_run")
		via := getStringSlice(cmd, "via")
		dest := args[0]

		ves := rep.HostedDeviceCode
		eta, err := common.Travel(ves, dest, dryRun, via...)
		if err != nil {
			return err
		}
		log("Travel initiated, ETA: %s (%s)", eta, time.Until(eta))
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
	// By accident, we prefer shorter names than longer - hub > vessel > matrix
	rows, err := db.DB.Query(`
	  SELECT type
	  FROM blueprints
	  WHERE 'cradle' = any(features)
	  ORDER BY length(type);`)
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
		if !slices.ContainsFunc(c.StowedDevices.Devices, func(d *models.DevicePointer) bool {
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
		getERM := func(d *models.Device) *models.CodeAlias {
			for _, s := range d.StowedDevices.Devices {
				if s.Type != "empty_replicant_matrix" {
					continue
				}
				return s.Code
			}
			return nil
		}
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		targetStr := getString(cmd, "target")
		var target *models.CodeAlias
		if targetStr == "" {
			loc := getString(cmd, "location")
			dests, err := getTeleportDests(loc)
			if err != nil {
				return err
			}
			for _, d := range dests {
				log("%s @ %s...", d.Code.Alias(), d.Location)
				if string(d.Location) == loc {
					log("...bullseye")
					target = getERM(d)
					break
				}
				if slices.Contains(d.Features, "cruise") && strings.HasPrefix(loc, d.Location.Star()) {
					log("...ship in system")
					target = getERM(d)
				}
			}
			if target == nil {
				return fmt.Errorf("No empty matrixes found at %s", loc)
			}
		} else {
			target = models.NewCodeAlias(targetStr)
		}
		res, err := rest.ReplicantTeleport(rID, target)
		if err != nil {
			return err
		}
		printTable([]string{
			"Replicant", "Status", "Source", "Destination", "Matrix", "Completes", "Online",
		}, [][]any{{
			rID, res.Status, res.SourceStar, res.DestinationStar, res.TargetMatrixCode,
			res.Completes.Time(), res.Completes.Time().Add(res.Offline.Duration()),
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
		var data [][]any
		for _, d := range dests {
			data = append(data, []any{
				d,
				d.Location,
				d.Type,
				d.StowedDevices.Devices[0],
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
