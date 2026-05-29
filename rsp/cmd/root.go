package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rsp",
	Short: "Simple cli for interacting with replicant.space",
	Run: runCmd.Run,
}

func log(tmpl string, args ...any) {
	fmt.Fprintf(os.Stderr, tmpl, args...)
}

func die(tmpl string, args ...any) {
	log("FATAL: "+tmpl, args...)
	os.Exit(1)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		die(err.Error())
	}
}
