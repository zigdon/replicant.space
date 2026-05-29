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
	Run: func(cmd *cobra.Command, args []string) {
		me, err := rest.Me()
		if err != nil {
			log("Error getting status: %v", err)
			return
		}
		fmt.Printf("%#v\n", me)
	},
}

func init() {
	rootCmd.AddCommand(meCmd)
}
