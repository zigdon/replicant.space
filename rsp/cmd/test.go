package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		rows, err := db.Query(`
			SELECT code, location, data->'cargo' cargo, data->'tags' tags
			FROM json_devices LEFT JOIN deliveries ON code = ship
			WHERE id IS NULL
			  AND type = 'cargo_freighter'
			  AND status = 'idle'
			  AND data->'tags' = '[]'
			  AND data->>'cargo' IS NOT NULL
			  AND data->'cargo' != '[]'
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		var data [][]any
		for rows.Next() {
			var c, l string
			var cargo cache.JSONB[[]*models.Inventory]
			var tags cache.JSONB[[]string]
			if err := rows.Scan(&c, &l, &cargo, &tags); err != nil {
				return err
			}
			data = append(data, []any{models.NewCodeAlias(c).Alias(), l, cargo.Data, tags.Data})
		}
		common.PrintTable([]string{"Code", "Location", "Cargo", "Tags"}, data)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
