package models

import (
	"cmp"
	"slices"
	"time"
)

type Trade struct {
	CreatedAt string `json:"created_at"`
	Created   time.Time
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
	TradeCode string `json:"trade_code"`
}

func (t *Trade) Fill() error {
	return fillTime(t.CreatedAt, &t.Created)
}

type Shop struct {
	ControllerCode     string   `json:"controller_code"`
	Description        string   `json:"description"`
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
	slices.SortFunc(s.Trades, func(a, b *Trade) int {
		return cmp.Compare(a.TradeCode, b.TradeCode)
	})
	for _, t := range s.Trades {
		if err := t.Fill(); err != nil {
			return err
		}
	}
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
