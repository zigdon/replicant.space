package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := db.QueryDevices(cache.QueryDevicesType("heaven_vessel"), cache.QueryDevicesStatus(args...))
		if err != nil {
			return err
		}
		var data [][]any
		for _, r := range res {
			dev, err := models.ParseOnly[models.Device](r.Data)
			if err != nil {
				return err
			}
			data = append(data, []any{r.Code, r.Type, r.Status, r.Location, dev.Code})
		}
		printTable([]string{"Code", "Type", "Status", "Location", "Alias"}, data)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
