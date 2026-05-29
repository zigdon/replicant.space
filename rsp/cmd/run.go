package cmd

import (
	tea "charm.land/bubbletea/v2"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/tui"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start an interactive menu",
	Run: func(cmd *cobra.Command, args []string) {
		p := tea.NewProgram(tui.Init())
		if _, err := p.Run(); err != nil {
			die("Error running TUI: %v", err)
		}

	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
