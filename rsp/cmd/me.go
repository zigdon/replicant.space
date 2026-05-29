package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
    "github.com/zigdon/rsp/rest"
	"github.com/zigdon/rsp/models"
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
		if raw {
			fmt.Println(res)
			return
		}
		printMe(res)
	},
}

func init() {
	rootCmd.AddCommand(meCmd)
}

func printMe(data []byte) {
	me, err := models.ParseMe(data)
	if err != nil {
		log("Error parsing me: %v", err)
		return
	}
	fmt.Printf("%#v\n", me)
}
