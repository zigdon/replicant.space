package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// retargetCmd represents the retarget command
var retargetCmd = &cobra.Command{
	Use:   "retarget",
	Short: "Change what resource a drone mines",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		r, _ := cmd.Flags().GetString("resource")
		resp, err := rest.DeviceCommand(id, "retarget", map[string]string{
			"resource_type": r,
		})
		if err != nil {
			return fmt.Errorf("Failed to retarget mining for %q: %v", r, err)
		}
		// if raw, _ := cmd.Flags().GetBool("raw"); raw { prettyPrint(resp) }
		prettyPrint(resp)
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(retargetCmd)
	retargetCmd.Flags().StringP("resource", "r", "", "Resource to mine")
	retargetCmd.MarkFlagRequired("resource")
}
