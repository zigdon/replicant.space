package cmd

import (
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:  "test",
	RunE: readStream,
}

func init() {
	rootCmd.AddCommand(testCmd)
}
