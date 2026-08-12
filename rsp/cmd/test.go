package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceType := args[0]
		rows, err := db.DB.Query("SELECT name FROM aliases WHERE type = $1", deviceType)
		if err != nil {
			return fmt.Errorf("Error getting existing aliases for %q: %v", deviceType, err)
		}
		var last int
		for rows.Next() {
			var a string
			if err := rows.Scan(&a); err != nil {
				return fmt.Errorf("Error scanning rows: %v", err)
				log("No existing aliases found for %q: %v", deviceType, err)
				continue
			}
			_, id, ok := strings.Cut(a, "-")
			if !ok {
				log("Invalid %q alias: %q", deviceType, a)
				continue
			}
			n, err := strconv.Atoi(id)
			if err != nil {
				log("Invalid %q alias: %q: %v", deviceType, a, err)
				continue
			}
			if n > last {
				last = n
			}
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("Error closing query: %v", err)
		}

		// prettyPrint(res)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
