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
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
		  prettyPrint(res)
		  return nil
		}
		if len(res.Traders) == 0 {
		  fmt.Println("No shops found.")
		  return nil
		}
		var shops [][]string
		for _, t := range res.Traders {
		  shops = append(shops, []string{
			t.ControllerCode, t.ShopName, t.OwnerName,
			wrap(t.Description, 40), t.Location, d(t.TradeCount),
			d(t.TotalStock),
		  })
		}
		printTable([]string{
		  "Code", "Name", "Owner", "Description", "Location", "Trades", "Stock",
		  }, shops)
		return nil
	},
}

var shopTradesCmd = &cobra.Command{
	Use:   "list",
	Short: "List trades of a specific shop",
	RunE: func(cmd *cobra.Command, args []string) error {
		sid, _ := cmd.Flags().GetString("shop")
		res, err := rest.Trades(sid)
		if err != nil {
		  return err
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
		  prettyPrint(res)
		  return nil
		}
		if len(res.Trades) == 0 {
		  fmt.Println("No trades found.")
		  return nil
		}
		var data [][]string
		for _, t := range res.Trades {
		  data = append(data, []string{
			t.Name, t.Code, d(t.CurrentStock),
			m(t.Criteria.Resources), m(t.Criteria.Devices),
			m(t.Rewards.Resources), m(t.Rewards.Devices),
		  })
		}

		printTable([]string{
		  "Name", "Code", "Stock", "Resource cost", "Device cost",
		  "Resource rewards", "Device rewards",
		}, data)
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(shopCmd)
	shopCmd.AddCommand(allShopCmd)
	shopCmd.AddCommand(shopTradesCmd)
	shopTradesCmd.Flags().StringP("shop", "s", "", "Shop code to list")
	shopTradesCmd.MarkFlagRequired("shop")
}
