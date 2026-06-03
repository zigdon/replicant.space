package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cfg"
	"github.com/zigdon/rsp/rest"

	"charm.land/huh/v2"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Create or edit the config",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := rest.Account()
		inst := "The current API key is: "
		if err == nil {
			inst += "VALID"
		} else {
			inst += fmt.Sprintf("INVALID (%v)", err)
		}
		inst += "\n\nEnter new API key, or leave empty to keep the existing one."
		var newKey string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(inst).
					Value(&newKey),
			),
		)
		if err := form.Run(); err != nil {
			die("Error running form: %v", err)
		}

		if len(strings.TrimSpace(newKey)) == 0 {
			log("API key unchanged")
			return nil
		}

		config, _ := cfg.ReadCfg()
		if config == nil {
			config = &cfg.Config{}
		}
		config.APIKey = newKey
		if err := cfg.UpdateCfg(config); err != nil {
			die("Failed to update config: %v", err)
		}
		log("Config updated.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
