package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(db.ChecksumHubs())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
