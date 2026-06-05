package cmd

import (
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

var cacheStarsCmd = &cobra.Command{
	Use: "reload-stars",
	Short: "Fetch the full star census to the cache",
	RunE: reloadStars,
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(cacheInitCmd)
	cacheInitCmd.Flags().Bool("create", false,
	  "Be willing to create a new db if none is found")
}

func reloadStars (cmd *cobra.Command, args []string) error {
	db, err := cache.Connect(false)
	if err != nil {
		return err
	}

	// Get the current list of stars.
	_, err = db.List("stars")
	if err != nil {
		return err
	}
}
