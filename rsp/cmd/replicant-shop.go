package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var shopCmd = &cobra.Command{
	Use:   "shop",
	Short: "List and interact with shops",
	RunE: allShopCmd.RunE,
}

var allShopCmd = &cobra.Command{
	Use:   "all",
	Short: "List all known shops",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		res, err := rest.Traders(rID)
		if err != nil {
		  return err
		}
		prettyPrint(res)
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(shopCmd)
	shopCmd.AddCommand(allShopCmd)
}
