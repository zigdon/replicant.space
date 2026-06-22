package models

import (
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
	if b.PrintTimeSeconds > 0 {
		return fillDuration(b.PrintTimeSeconds, &b.PrintTime)
	}
	return nil
}

type Blueprints struct {
	Blueprints []*Blueprint `json:"blueprints"`
}

func (bs *Blueprints) Fill() error {
	return fill([]fillData{{recurse: f(bs.Blueprints)}})
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
	if pr.PrintTimeSeconds > 0 {
		return fillDuration(pr.PrintTimeSeconds, &pr.PrintTime)
	}
	return nil
}

type Queued struct {
	Queue       []string `json:"queue"`
	QueueLength int      `json:"queue_length"`
	Status      string   `json:"status"`
}
