package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the sqlite cache",
}

var cacheInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create the db or update the schema",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := cache.Connect(true)
		if err != nil {
			return err
		}
		log("cache updated: %s", db.Stats())
		return nil
	},
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cache stats",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := cache.Connect(false)
		if err != nil {
			return err
		}
		log("cache stats: %s", db.Stats())
		return nil
	},
}

var updateSchemaCmd = &cobra.Command{
	Use:   "update-schema",
	Short: "Update the database schema",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := cache.Connect(false)
		if err != nil {
			return err
		}
		return db.UpdateSchema()
	},
}

var reloadStarsCmd = &cobra.Command{
	Use:   "reload-stars",
	Short: "Fetch the full star census to the cache",
	RunE:  reloadStars,
}

var resetUniverseCmd = &cobra.Command{
	Use:   "reset-universe",
	Short: "Clear all the universe cache tables",
	RunE:  resetUniverse,
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(cacheInitCmd)
	cacheInitCmd.Flags().Bool("create", false,
		"Be willing to create a new db if none is found")
	cacheCmd.AddCommand(reloadStarsCmd)
	cacheCmd.AddCommand(statsCmd)
	cacheCmd.AddCommand(updateSchemaCmd)
	cacheCmd.AddCommand(resetUniverseCmd)
	resetUniverseCmd.Flags().Bool("delete", false, "Confirm that all the data should be deleted")
	resetUniverseCmd.MarkFlagRequired("delete")
}

func resetUniverse(cmd *cobra.Command, args []string) error {
	for _, t := range []cache.Tables{
		cache.StarsTable,
		cache.PlanetsTable,
		cache.MoonsTable,
		cache.BeltsTable,
		cache.BeltResTable,
		cache.BlueprintsTable,
		cache.BlueprintResTable,
		cache.BlueprintDirsTable,
		cache.BlueprintFeaturesTable,
	} {
		if err := db.Reset(t); err != nil {
			return fmt.Errorf("Couldn't clear %s: %v", t, err)
		}
	}
	return nil
}

func reloadStars(cmd *cobra.Command, args []string) error {
	db, err := cache.Connect(false)
	if err != nil {
		return err
	}

	// Get the current list of stars.
	seen := make(map[string]*models.Star)
	oldStars, err := db.ListIDs(cache.StarsTable)
	if err != nil {
		return err
	}
	log("%d stars loaded from the cache", len(oldStars))
	for _, id := range cache.Strs(oldStars) {
		s := &models.Star{Designation: id}
		if err := s.Get(); err != nil {
			return err
		}
		seen[id] = s
	}

	// Get a replicant ID
	id, err := rest.ReplicantID(1)
	if err != nil {
		return err
	}

	// Get page 1, and also how many pages there are
	page := 0
	var added, updated []*models.Star
	unchanged := 0
	for {
		census, err := rest.ReplicantCensus(id, 50, page)
		if err != nil {
			return err
		}
		for _, star := range census.Stars {
			if old, ok := seen[star.Designation]; ok {
				if old == star {
					unchanged++
					continue
				}
				updated = append(updated, star)
			} else {
				added = append(added, star)
			}
			seen[star.Designation] = star

		}
		log(
			"Page %d/%d: %d total stars, %d added, %d updated, %d unchanged",
			page, census.TotalPages, census.TotalStars, len(added),
			len(updated), unchanged)
		page++
		if page > census.TotalPages {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	log(
		"Fetch done: %d total stars, %d added, %d updated, %d unchanged",
		len(added)+len(updated)+unchanged, len(added), len(updated), unchanged)
	for _, s := range append(added, updated...) {
		if err := s.Cache(); err != nil {
			return err
		}
	}
	log("Updated done.")
	return nil
}
