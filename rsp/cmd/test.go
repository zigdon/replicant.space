package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _ := db.ChecksumHubs()
		fmt.Println(cache.Encode(c))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
