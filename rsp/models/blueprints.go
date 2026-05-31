package models

import (
	"fmt"

	"encoding/json"
)

type Blueprint struct {
	AttachCapacity int `json:"attach_capacity"`
	CargoCapacity int `json:"cargo_capacity"`
	DeviceType string `json:"device_type"`
	Directives []string `json:"directives"`
	Features []string `json:"features"`
	PrintTime float32 `json:"print_time"`
	Resources map[string]int `json:"resources"`
	StowCapacity int `json:"stow_capacity"`
}

type Blueprints struct {
	Blueprints []Blueprint `json:"blueprints"`
}

func ParseBlueprints(data []byte) (*Blueprints, error) {
	bs := &Blueprints{}
	if err := json.Unmarshal(data, bs); err != nil {
		return nil, fmt.Errorf("Error parsing device list: %v", err)
	}

	return bs, nil
}
