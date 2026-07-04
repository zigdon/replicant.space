package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := getInfo(models.NewCodeAlias(args[0]))
		if err != nil {
			return err
		}
		eta := getETA(p)
		prettyPrint(eta)
		log("fetched: %s", eta.Device.Fetched())
		log("zero: %v", eta.Device.Fetched().IsZero())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
