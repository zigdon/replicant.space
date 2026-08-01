package cmd

import (
	"errors"
	"fmt"
	"strconv"

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
		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(res)
			return nil
		}
		if len(res.Traders) == 0 {
			fmt.Println("No shops found.")
			return nil
		}
		var shops [][]any
		for _, t := range res.Traders {
			shops = append(shops, []any{
				t.ControllerCode, t.ShopName, t.OwnerName,
				wrap(t.Description, 40), t.Location, t.TradeCount, t.TotalStock,
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
		sid := getString(cmd, "shop")
		res, err := rest.Trades(sid)
		if err != nil {
			return err
		}
		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(res)
			return nil
		}
		if len(res.Trades) == 0 {
			fmt.Println("No trades found.")
			return nil
		}
		fmt.Printf("Trades listed for %s:\n", sid)
		printTrades(res.Trades)
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
		tid := getString(cmd, "trade")
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
			[][]any{{m(trade.Rewards.Devices), m(trade.Rewards.Resources)}})
		return nil
	},
}

var addTradeCmd = &cobra.Command{
	Use:   "add",
	Short: "Offer a new trade",
	RunE: func(cmd *cobra.Command, args []string) error {
		sid := getString(cmd, "shop")
		name := getString(cmd, "name")
		stock := getInt(cmd, "stock")
		costR := getMap(cmd, "cost_res")
		costD := getMap(cmd, "cost_dev")
		sellR := getMap(cmd, "sell_res")
		sellD := getMap(cmd, "sell_dev")

		var errs []error
		mkStrIntMap := func(in map[string]string) map[string]int {
			res := make(map[string]int)
			for k, v := range in {
				n, err := strconv.Atoi(v)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				res[k] = n
			}
			return res
		}
		cfg := map[string]any{
			"name":  name,
			"stock": stock,
			"criteria": map[string]any{
				"resources": mkStrIntMap(costR),
				"devices":   mkStrIntMap(costD),
			},
			"rewards": map[string]any{
				"resources": mkStrIntMap(sellR),
				"devices":   mkStrIntMap(sellD),
			},
		}
		if len(errs) > 0 {
			return errors.Join(errs...)
		}
		res, err := rest.Sell(models.NewCodeAlias(sid), cfg)
		if err != nil {
			return err
		}

		fmt.Println("Trade posted")
		printTrades([]*models.Trade{res})
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

func printTrades(trades []*models.Trade) {
	var data [][]any
	for _, t := range trades {
		data = append(data, []any{
			wrap(t.Name, 20), t.Code, t.CurrentStock,
			m(t.Criteria.Resources), m(t.Criteria.Devices),
			m(t.Rewards.Resources), m(t.Rewards.Devices),
		})
	}

	printTable([]string{
		"Name", "Code", "Stock", "Resource cost", "Device cost",
		"Resource rewards", "Device rewards",
	}, data)
}
