package cmd

import (
	"fmt"
	"strconv"

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

var test2Cmd = &cobra.Command{
	Use: "2",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := &models.Star{Designation: args[0]}
		s.Get()
		log("pos=%v", s.Position)
		cone, _ := strconv.Atoi(args[1])
		margin, _ := strconv.Atoi(args[2])
		res, err := db.GetSector(
			s.Position.X, s.Position.Y, s.Position.Z, cone, margin,
		)
		if err != nil {
			return err
		}
		log("%v", res)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
	testCmd.AddCommand(test2Cmd)
}
