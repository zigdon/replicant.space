package cmd

import (
	"fmt"

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
	out, err := rest.ReloadStars()
	if err != nil {
		return err
	}
	log(out)
	return nil
}
