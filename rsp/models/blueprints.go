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

type PrintResp struct {
	Status string `json:"strin"`
	DeviceType string `json:"device_type"`
	StartedAt string `json:"started_at"`
	CompletesAt string `json:"completes_at"`
	PrintTimeSeconds int `json:"print_time_seconds"`
	ResourcesRefunded bool `json:"resources_refunded"`
}

type Queued struct {
	Queue []string `json:"queue"`
	QueueLength int `json:"queue_length"`
	Status string `json:"status"`
}

func ParseBlueprints(data []byte) (*Blueprints, error) {
	bs := &Blueprints{}
	if err := json.Unmarshal(data, bs); err != nil {
		return nil, fmt.Errorf("Error parsing device list: %v", err)
	}

	return bs, nil
}

func ParsePrintResp(data []byte) (*PrintResp, error) {
	pr := &PrintResp{}
	if err := json.Unmarshal(data, pr); err != nil {
		return nil, fmt.Errorf("Error parsing print response: %v", err)
	}

	return pr, nil
}

func ParseQueued(data []byte) (*Queued, error) {
	q := &Queued{}
	if err := json.Unmarshal(data, q); err != nil {
		return nil, fmt.Errorf("Error parsing queue list: %v", err)
	}

	return q, nil
}
