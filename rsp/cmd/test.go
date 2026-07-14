package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := rest.ProspectLogs(models.NewCodeAlias(args[0]))
		log(out)
		return err
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
