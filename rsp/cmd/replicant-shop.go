package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var shopCmd = &cobra.Command{
	Use:   "shop",
	Short: "List and interact with shops",
	RunE:  allShopCmd.RunE,
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

		fmt.Printf("Trades listed for %s:\n", sid)
		printTable([]string{
			"Name", "Code", "Stock", "Resource cost", "Device cost",
			"Resource rewards", "Device rewards",
		}, data)
		return nil
	},
}

var executeTradeCmd = &cobra.Command{
	Use:   "trade",
	Short: "Do a trade",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		tid, _ := cmd.Flags().GetString("trade")
		// Find the shop controller for this trade
		shops, err := rest.Traders(rID)
		if err != nil {
			return err
		}
		var cid string
		var trade *models.Trade
		for _, s := range shops.Traders {
			trades, err := rest.Trades(s.ControllerCode)
			if err != nil {
				return err
			}
			for _, t := range trades.Trades {
				if t.Code == tid {
					trade = t
					cid = s.ControllerCode
					break
				}
			}
			if cid != "" {
				break
			}
		}

		if cid == "" {
			return fmt.Errorf("Can't find a shop for %q", tid)
		}

		_, err = rest.Trade(cid, tid)
		if err != nil {
			return err
		}
		printTable([]string{"Devices", "Resources"},
			[][]string{{m(trade.Rewards.Devices), m(trade.Rewards.Resources)}})
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(shopCmd)
	shopCmd.AddCommand(allShopCmd)
	shopCmd.AddCommand(shopTradesCmd)
	shopTradesCmd.Flags().StringP("shop", "s", "", "Shop code to list")
	shopTradesCmd.MarkFlagRequired("shop")

	shopCmd.AddCommand(executeTradeCmd)
	executeTradeCmd.Flags().StringP("trade", "t", "", "Trade ID")
	executeTradeCmd.MarkFlagRequired("trade")
}
