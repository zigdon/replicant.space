package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rsp",
	Short: "Simple cli for interacting with replicant.space",
	Run: runCmd.Run,
}

func log(tmpl string, args ...any) {
	if !strings.HasSuffix(tmpl, "\n") {
		tmpl += "\n"
	}
	fmt.Fprintf(os.Stderr, tmpl, args...)
}

func die(tmpl string, args ...any) {
	log("FATAL: "+tmpl, args...)
	os.Exit(1)
}

func prettyPrint(i any) {
    s, _ := json.MarshalIndent(i, "", "  ")
    fmt.Println(string(s))
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		die(err.Error())
	}
}
