package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// mineCmd represents the mine command
var mineCmd = &cobra.Command{
	Use:   "mine",
	Short: "Instruct a drone to start mining",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		r, _ := cmd.Flags().GetString("resource")
		resp, err := rest.DeviceCommand(id, "start_mining", map[string]string{
			"resource_type": r,
		})
		if err != nil {
			return fmt.Errorf("Failed to start mining for %q: %v", r, err)
		}
		// if raw, _ := cmd.Flags().GetBool("raw"); raw { prettyPrint(resp) }
		prettyPrint(resp)
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(mineCmd)
	mineCmd.Flags().StringP("resource", "r", "", "Resource to mine")
	mineCmd.MarkFlagRequired("resource")
}
