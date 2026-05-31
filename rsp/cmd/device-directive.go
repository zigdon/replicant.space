package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// retargetCmd represents the retarget command
var directiveCmd = &cobra.Command{
	Use:   "set_directive",
	Short: "Update the automation policy for a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		n, _ := cmd.Flags().GetString("new_directive")
		resp, err := rest.DeviceCommand(id, "set_directive", map[string]string{
			"directive": n,
		})
		if err != nil {
			return fmt.Errorf("Failed to update directive for %q: %v", n, err)
		}
		// if raw, _ := cmd.Flags().GetBool("raw"); raw { prettyPrint(resp) }
		prettyPrint(resp)
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(directiveCmd)
	directiveCmd.Flags().StringP("new_directive", "n", "", "New directive")
	directiveCmd.MarkFlagRequired("new_directive")
}
