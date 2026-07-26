package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: readStream,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func readStream(cmd *cobra.Command, args []string) error {
	handle := func(ev map[string]string) error {
		prettyPrint(ev)
		return nil
	}

	if err := rest.ReadStream(handle); err != nil {
		return err
	}

	return nil
}
