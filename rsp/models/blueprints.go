package models

import (
	"fmt"
	"time"
)

type Blueprint struct {
	AttachCapacity   int      `json:"attach_capacity"`
	CargoCapacity    int      `json:"cargo_capacity"`
	DeviceType       string   `json:"device_type"`
	Directives       []string `json:"directives"`
	Features         []string `json:"features"`
	PrintTimeSeconds float32  `json:"print_time"`
	PrintTime        time.Duration
	Resources        map[string]int `json:"resources"`
	StowCapacity     int            `json:"stow_capacity"`
}

func (b *Blueprint) Fill() error {
	var err error
	if b.PrintTimeSeconds > 0 {
		b.PrintTime, err = time.ParseDuration(fmt.Sprintf("%.2fs", b.PrintTimeSeconds))
	}
	return err
}

type Blueprints struct {
	Blueprints []*Blueprint `json:"blueprints"`
}

func (bs *Blueprints) Fill() error {
	for _, b := range bs.Blueprints {
		if err := b.Fill(); err != nil {
			return err
		}
	}
	return nil
}

type PrintResp struct {
	Status            string  `json:"status"`
	DeviceType        string  `json:"device_type"`
	StartedAt         string  `json:"started_at"`
	CompletesAt       string  `json:"completes_at"`
	PrintTimeSeconds  float32 `json:"print_time_seconds"`
	PrintTime         time.Duration
	ResourcesRefunded bool `json:"resources_refunded"`
}

func (pr *PrintResp) Fill() error {
	var err error
	if pr.PrintTimeSeconds > 0 {
		pr.PrintTime, err = time.ParseDuration(fmt.Sprintf("%.2fs", pr.PrintTimeSeconds))
	}
	return err
}

type Queued struct {
	Queue       []string `json:"queue"`
	QueueLength int      `json:"queue_length"`
	Status      string   `json:"status"`
}
