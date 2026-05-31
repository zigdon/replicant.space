package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// deployCmd represents the deploy command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Initiate a search",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		resp, err := rest.DeviceCommand(id, "search", nil)
		if err != nil {
			return fmt.Errorf("%q search failed: %v", id, err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(resp)
		} else {
			eta, _ := time.ParseDuration(fmt.Sprintf("%fs", resp.EtaSeconds))
			printTable(
				[]string{"Code", "Belt", "ETA", "Status"},
				[][]string{{resp.DeviceCode, resp.Belt, eta.String(), resp.Status}},
				0,
			)
		}
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(searchCmd)
}
