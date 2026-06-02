package cmd

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/tui"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start an interactive menu",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(tui.InitModel())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("Error running TUI: %v", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
