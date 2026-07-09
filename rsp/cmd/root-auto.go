package cmd

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var autoCmd = &cobra.Command{
	Use:   "auto",
	Short: "High level automation commands",
}

var autoMineCmd = &cobra.Command{
	Use:   "mine",
	Short: "Set up a new belt mine",
	RunE:  autoMine,
}

var autoFerryCmd = &cobra.Command{
	Use:   "ferry",
	Short: "Ferry resources to the home belt",
	RunE:  autoFerry,
}

var autoRelayCmd = &cobra.Command{
	Use:   "relay",
	Short: "Set up relay network",
	RunE:  autoRelay,
}

func init() {
	rootCmd.AddCommand(autoCmd)

	autoCmd.PersistentFlags().Bool("dry_run", true, "When set, only describe what will be done")
	autoCmd.PersistentFlags().String("owner", "zigdon-2", "Replicant responsible for printing new devices")

	autoCmd.AddCommand(autoMineCmd)
	autoMineCmd.Flags().StringP("location", "l", "", "Belt location to mine")
	autoMineCmd.MarkFlagRequired("location")
	autoMineCmd.Flags().StringSliceP("factory", "f", []string{}, "If listed, use only these factories")
	autoMineCmd.Flags().BoolP("dry_run", "n", false, "Only plan, don't actually queue prints")
	autoMineCmd.Flags().String("fleet", "afc-1", "Fleet controller to use for transportation")
	autoMineCmd.Flags().Bool("no_print", false, "Skip printing missing resources")
	autoMineCmd.Flags().StringSlice("skip", []string{}, "Remove these devices from the plan")
	autoMineCmd.Flags().String("home", "MENKUNT-BELT-1", "Destination for ferrying")

	autoCmd.AddCommand(autoFerryCmd)
	autoFerryCmd.Flags().String("home", "MENKUNT-BELT-1", "Destination for ferrying")
	autoFerryCmd.Flags().StringP("atc", "t", "atc-1", "Transport controller to use")

	autoCmd.AddCommand(autoRelayCmd)
	autoRelayCmd.Flags().String("home", "MENKUNT", "Home system")
}

var infos sync.Map

func getInfo(d *models.CodeAlias) (*models.Device, error) {
	i, ok := infos.Load(d)
	if ok {
		return i.(*models.Device), nil
	}
	i, err := rest.DeviceInfo(d)
	if err != nil {
		return nil, err
	}
	infos.Store(d, i)
	return i.(*models.Device), nil
}

func travel(id *models.CodeAlias, location string) error {
	info, err := getInfo(id)
	if err != nil {
		return fmt.Errorf("Can't get %s info: %v", id.Alias(), err)
	}
	if info.Location != location {
		res, err := rest.DeviceCommand(id, "travel", map[string]any{
			"destination": location,
		})
		if err != nil {
			return fmt.Errorf("Failed to send %s from %q to %q: %v", id.Alias(), info.Location, location, err)
		}
		log("Shipped %s to %s: ETA %s", id.Alias(), location, res.TotalTime.String())
	}
	return nil
}

func setDirective(id *models.CodeAlias, directive string, cfg map[string]any) error {
	if _, err := rest.DeviceCommand(id, "set_directive", map[string]any{
		"directive":     directive,
		"configuration": cfg,
	}); err != nil {
		return fmt.Errorf("Can't set %s directive: %v", id.Alias(), err)
	}
	if directive != "patrol" {
		if _, err := rest.DeviceCommand(id, "launch", nil); err != nil {
			return fmt.Errorf("Can't launching %s: %v", id.Alias(), err)
		}
	}
	log("Set directive on %s: %s", id.Alias(), directive)
	return nil
}
