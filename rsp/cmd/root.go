package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var raw bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rsp",
	Short: "Simpler cli for interacting with replicant.space",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

func log(tmpl string, args ...any) {
	fmt.Fprintf(os.Stderr, tmpl, args...)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&raw, "raw", false, "enable verbose output")
}
