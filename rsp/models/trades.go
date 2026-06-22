package models

import (
	"cmp"
	"fmt"
	"slices"
)

type Trade struct {
	Code      string `json:"trade_code"`
	Created   JSONTime `json:"created_at"`
	Criteria  struct {
		Devices   map[string]int `json:"devices"`
		Resources map[string]int `json:"resources"`
	} `json:"criteria"`
	CurrentStock int    `json:"current_stock"`
	InitialStock int    `json:"initial_stock"`
	Name         string `json:"name"`
	Rewards      struct {
		Devices   map[string]int `json:"devices"`
		Resources map[string]int `json:"resources"`
	} `json:"rewards"`
}

type Shop struct {
	ControllerCode     string   `json:"controller_code"`
	Description        string   `json:"description"`
	Error              string   `json:"error"`
	IsLocal            bool     `json:"is_local"`
	Location           string   `json:"location"`
	LocationName       string   `json:"location_name"`
	OwnerName          string   `json:"owner_name"`
	OwnerReplicantCode string   `json:"owner_replicant_code"`
	ShopName           string   `json:"shop_name"`
	Star               string   `json:"star"`
	TotalStock         int      `json:"total_stock"`
	TradeCount         int      `json:"trade_count"`
	Trades             []*Trade `json:"trades"`
}

func (s *Shop) Fill() error {
	if s.Error != "" {
		return fmt.Errorf("shop error: %s", s.Error)
	}
	slices.SortFunc(s.Trades, func(a, b *Trade) int {
		return cmp.Compare(a.Code, b.Code)
	})
	return nil
}

type Shops struct {
	Traders []*Shop `json:"traders"`
}

func (ss *Shops) Fill() error {
	slices.SortFunc(ss.Traders, func(a, b *Shop) int {
		return cmp.Compare(a.ControllerCode, b.ControllerCode)
	})
	for _, s := range ss.Traders {
		if err := s.Fill(); err != nil {
			return err
		}
	}
	return nil
}
