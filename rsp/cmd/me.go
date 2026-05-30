package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
    "github.com/zigdon/rsp/rest"
)

// meCmd represents the me command
var meCmd = &cobra.Command{
	Use:   "me",
	Short: "Show current status",
	RunE: func(cmd *cobra.Command, args []string) error {
		me, err := rest.Account()
		if err != nil {
			return fmt.Errorf("Error getting status: %v", err)
		}
		prettyPrint(me)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(meCmd)
}
