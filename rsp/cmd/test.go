package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/auto"
	"github.com/zigdon/rsp/models"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		dev, err := getInfo(models.NewCodeAlias(args[0]))
		if err != nil {
			return err
		}
		m := &auto.ProspectMachine{}
		log("Start:")
		if err = m.Start(dev, true); err != nil {
			return fmt.Errorf("Start err: %v", err)
		}
		log(m.Status())
		log("Process")
		if t, err := m.Process(); err != nil {
			return fmt.Errorf("Start err: %v", err)
		} else {
			log("ETA: %s\n", t.String())
			log(m.Status())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
