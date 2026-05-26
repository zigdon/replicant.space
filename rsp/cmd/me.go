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
		res, err := rest.Get("accounts/me")
		if err != nil {
			log("Error getting status: %v", err)
			return
		}
		fmt.Println(res)
	},
}

func init() {
	rootCmd.AddCommand(meCmd)
}
