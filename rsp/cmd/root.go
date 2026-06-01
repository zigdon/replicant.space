package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	lg "charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
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

func b(n bool) string {
	if n { return "True" }
	return "False"
}

func f(n float32) string {
	return fmt.Sprintf("%.2f", n)
}

func d(n int) string {
	return fmt.Sprintf("%d", n)
}

func list(s []string) string {
	return strings.Join(s, ", ")
}

func m(in map[string]string) string {
	var res []string
	for k, v := range in {
		res = append(res, fmt.Sprintf("%s: %s", k, v))
	}
	return strings.Join(res, "\n")
}

func p(per float32) string {
	return fmt.Sprintf("%.2f%%", per*100)
}

func printTable(headers []string, data [][]string, width int) {
	if width == 0 { width = 20 }
	headerStyle  := lg.NewStyle().Bold(true).Align(lg.Center)
	cellStyle    := lg.NewStyle().Padding(0, 1).Width(width)

	t := table.New().
		Border(lg.NormalBorder()).
		StyleFunc(func(row, col int) lg.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		}).
		Headers(headers...).
		Rows(data...)
	lg.Println(t)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		die(err.Error())
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("raw", false, "emit the json returned")
}
