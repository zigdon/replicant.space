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

var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage aliases",
}

var aliasAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new alias for a device type",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("Missing arguments: rsp alias add <type> <alias>")
		}
		t := args[0]
		if a := db.GetPrefixForType(t); a != "" {
			return fmt.Errorf("%q already has a prefix: %q", t, a)
		}
		a := args[1]
		if t := db.GetTypeForPrefix(a); t != "" {
			return fmt.Errorf("%q is already a prefix: %q", a, t)
		}

		return db.AddAliasType(a, t)
	},
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

	cacheCmd.AddCommand(aliasCmd)
	aliasCmd.AddCommand(aliasAddCmd)
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
	// Get the current list of stars.
	seen := make(map[string]*models.Star)
	rows, err := db.DB.Query(`
		SELECT designation, name, explored, est_planets, has_life, position_x, position_y, position_z
		FROM stars`)
	if err != nil {
		return err
	}
	old := 0
	for rows.Next() {
		old++
		s := new(models.Star)
		var x, y, z float32
		err := rows.Scan(&s.Designation, &s.Name, &s.Explored, &s.EstimatedPlanets, &s.HasLife, &x, &y, &z)
		if err != nil {
			return err
		}
		s.Position = models.NewPosition(x, y, z)
		seen[s.Designation] = s
	}
	log("Loaded %d stars from cache", old)

	// Get a replicant ID, doesn't matter which
	id, err := rest.ReplicantID(1)
	if err != nil {
		return err
	}

	// Get page 1, and also how many pages there are
	page := 0
	var added, updated []*models.Star
	updatedStar := func(a, b *models.Star) bool {
		if a.Designation != b.Designation {
			log("Comparing different stars %q and %q", a.Designation, b.Designation)
			return true
		}
		if a.Name != b.Name {
			log("%q: name updated: %q -> %q", a.Designation, a.Name, b.Name)
			return true
		}
		if a.Explored != b.Explored {
			log("%q: explore updated: %v -> %v", a.Designation, a.Explored, b.Explored)
			return true
		}
		if a.EstimatedPlanets != b.EstimatedPlanets {
			log("%q: est planets updated: %d -> %d", a.Designation, a.EstimatedPlanets, b.EstimatedPlanets)
			return true
		}
		if a.HasLife != b.HasLife {
			log("%q: has life updated: %v -> %v", a.Designation, a.HasLife, b.HasLife)
			return true
		}
		return false
	}
	unchanged := 0
	for {
		census, err := rest.ReplicantCensus(id, 50, page)
		if err != nil {
			return err
		}
		for _, star := range census.Stars {
			if old, ok := seen[star.Designation]; ok {
				if !updatedStar(old, star) {
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
		time.Sleep(150 * time.Millisecond)
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
