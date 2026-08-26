package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(os.Args[0])

		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
