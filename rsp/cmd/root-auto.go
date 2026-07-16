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

var autoProspectCmd = &cobra.Command{
	Use:   "prospect",
	Short: "Continue prospecting towards predetermined goals",
	RunE:  autoProspect,
}

var autoRelayCmd = &cobra.Command{
	Use:   "relay",
	Short: "Set up relay network",
	RunE:  autoRelay,
}

var autoRentCmd = &cobra.Command{
	Use:   "rent",
	Short: "Ensure all system hubs have required upkeep resources available",
	RunE:  autoRent,
}

var autoFRCmd = &cobra.Command{
	Use:   "fr",
	Short: "Deploy, activate, and tag a new FTL relay",
	RunE:  autoFR,
}

var autoEventCmd = &cobra.Command{
	Use:   "event",
	Short: "Build and deliver event requirements",
	RunE:  autoEvent,
}

func init() {
	rootCmd.AddCommand(autoCmd)

	autoCmd.PersistentFlags().Bool("dry_run", true, "When set, only describe what will be done")
	autoCmd.PersistentFlags().String("owner", "zigdon-4", "Replicant responsible for printing new devices")

	autoCmd.AddCommand(autoMineCmd)
	autoMineCmd.Flags().StringP("location", "l", "", "Belt location to mine")
	autoMineCmd.MarkFlagRequired("location")
	autoMineCmd.Flags().StringSliceP("factory", "f", []string{}, "If listed, use only these factories")
	autoMineCmd.Flags().BoolP("dry_run", "n", false, "Only plan, don't actually queue prints")
	autoMineCmd.Flags().Bool("no_print", false, "Skip printing missing resources")
	autoMineCmd.Flags().StringSlice("skip", []string{}, "Remove these devices from the plan")
	autoMineCmd.Flags().String("home", "MENKUNT-2-L4", "Destination for ferrying")

	autoCmd.AddCommand(autoFerryCmd)
	autoFerryCmd.Flags().String("home", "MENKUNT-2-L4", "Destination for ferrying")
	autoFerryCmd.Flags().StringP("atc", "t", "atc-1", "Transport controller to use")

	autoCmd.AddCommand(autoRelayCmd)
	autoRelayCmd.Flags().String("home", "MENKUNT", "Home system")

	autoCmd.AddCommand(autoFRCmd)
	autoFRCmd.Flags().IntP("replicant", "r", 0, "Which replicant is deploying the FR")
	autoFRCmd.Flags().Bool("dry_run", true, "When set, only describe what will be done")
	autoFRCmd.MarkFlagRequired("replicant")

	autoCmd.AddCommand(autoProspectCmd)
	autoProspectCmd.Flags().StringSliceP("device", "d", []string{}, "Devices to use, leave blank for all")
	autoProspectCmd.Flags().BoolP("dry_run", "n", false, "Only log what actions would happen")

	autoCmd.AddCommand(autoRentCmd)
	autoRentCmd.Flags().StringP("atc", "t", "atc-3", "ATC controlling cargo transports")
	autoRentCmd.Flags().String("home", "MENKUNT-2-L4", "Home system")
	autoRentCmd.Flags().BoolP("dry_run", "n", false, "Only log what actions would happen")

	autoCmd.AddCommand(autoEventCmd)
	autoEventCmd.Flags().StringP("event_id", "e", "", "Event ID to work towards")
	autoEventCmd.Flags().String("home", "MENKUNT-2-L4", "Home system")
	autoEventCmd.Flags().IntP("criteria", "c", 0, "Which of multiple event criteria should be used")
	autoEventCmd.MarkFlagRequired("event_id")
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

func resetInfo(d *models.CodeAlias) {
	infos.Delete(d)
}

func travel(id *models.CodeAlias, location string) error {
	info, err := getInfo(id)
	if err != nil {
		return fmt.Errorf("Can't get %s info: %v", id.Alias(), err)
	}
	if string(info.Location) != location {
		res, err := rest.DeviceCommand[models.CommandResp](id, "travel", map[string]any{
			"destination": location,
		})
		if err != nil {
			return fmt.Errorf("Failed to send %s from %q to %q: %v", id.Alias(), info.Location, location, err)
		}
		log("Shipped %s to %s: ETA %s", id.Alias(), location, res.TotalTime.String())
	}
	resetInfo(id)
	return nil
}

func setDirective(id *models.CodeAlias, directive string, cfg map[string]any) error {
	if _, err := rest.DeviceCommand[models.CommandResp](id, "set_directive", map[string]any{
		"directive":     directive,
		"configuration": cfg,
	}); err != nil {
		return fmt.Errorf("Can't set %s directive: %v", id.Alias(), err)
	}
	if directive != "patrol" {
		if _, err := rest.DeviceCommand[models.CommandResp](id, "launch", nil); err != nil {
			return fmt.Errorf("Can't launching %s: %v", id.Alias(), err)
		}
	}
	log("Set directive on %s: %s", id.Alias(), directive)
	resetInfo(id)
	return nil
}

func adopt(cnt *models.CodeAlias, minions []*models.CodeAlias) error {
	log("Adopting %v into %q", aliases(minions), cnt)
	var ids []string
	for _, m := range minions {
		ids = append(ids, m.String())
		resetInfo(m)
	}
	_, err := rest.DeviceCommand[models.CommandResp](cnt, "adopt", map[string]any{
		"devices": minions,
	})
	resetInfo(cnt)
	return err
}
