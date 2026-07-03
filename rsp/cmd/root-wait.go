package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var waitCmd = &cobra.Command{
	Use: "wait",
	Short: "Follow all pending tasks",
	RunE: waitPending,
}

func init() {
	rootCmd. AddCommand(waitCmd)
}

func waitPending(cmd *cobra.Command, args []string) error {
	devs, err := rest.Devices(nil)
	if err != nil {
		return err
	}
	for _, d := range devs {
		log("%s: %s - %v", d.Code.Alias(), d.Status, d.Travel)
	}

	return nil
}
