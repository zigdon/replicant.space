package cmd

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
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
	Use: "stats",
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
	Use: "update-schema",
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
	Use: "reload-stars",
	Short: "Fetch the full star census to the cache",
	RunE: reloadStars,
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(cacheInitCmd)
	cacheInitCmd.Flags().Bool("create", false,
	  "Be willing to create a new db if none is found")
	cacheCmd.AddCommand(reloadStarsCmd)
	cacheCmd.AddCommand(statsCmd)
	cacheCmd.AddCommand(updateSchemaCmd)
}

func reloadStars (cmd *cobra.Command, args []string) error {
	db, err := cache.Connect(false)
	if err != nil {
		return err
	}

	// Get the current list of stars.
	seen := make(map[string]*cache.Star)
	oldStars, err := db.List("stars")
	if err != nil {
		return err
	}
	log("%d stars loaded from the cache", len(oldStars))
	for _, s := range oldStars {
		seen[s.(*cache.Star).Designation] = s.(*cache.Star)
	}

	// Get a replicant ID
	id, err := rest.ReplicantID(1)
	if err != nil {
		return err
	}

	// Get page 1, and also how many pages there are
	page := 0
	var added, updated []string
	unchanged := 0
	for {
		census, err := rest.ReplicantCensus(id, 50, page)
		if err != nil {
			return err
		}
		for _, star := range census.Stars {
			ns := &cache.Star{
				Designation: star.Designation,
				EntryPoint: star.EntryPoint,
				EstPlanets: star.EstimatedPlanets,
				Explored: star.Explored,
				HasLife: star.HasLife,
				Name: star.Name,
				PositionX: star.Position.X,
				PositionY: star.Position.Y,
				PositionZ: star.Position.Z,
			}
			if old, ok := seen[ns.Designation]; ok {
				if old.Equal(ns) {
					unchanged++
					continue
				}
				updated = append(updated, ns.Designation)
			} else {
				added = append(added, ns.Designation)
			}
			seen[ns.Designation] = ns

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
		len(added) + len(updated) + unchanged, len(added), len(updated), unchanged)
	for _, s := range append(added, updated...) {
		if err := db.Update(cache.StarsTable, seen[s].Map()); err != nil {
			return err
		}
	}
	log("Updated done.")
	return nil
}
