package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/zigdon/rsp/cache"
)

type Blueprint struct {
	AttachCapacity   int            `json:"attach_capacity"`
	CargoCapacity    int            `json:"cargo_capacity"`
	Components       map[string]int `json:"components"`
	DeviceType       string         `json:"device_type"`
	Description      string         `json:"description"`
	Directives       []string       `json:"directives"`
	Features         []string       `json:"features"`
	PrintTime        *JSONTimeDelta `json:"print_time"`
	Resources        map[string]int `json:"resources"`
	ShortDescription string         `json:"short_description"`
	StowCapacity     int            `json:"stow_capacity"`
	Strength         float32        `json:"strength"`
	Alias            string
}

func (b *Blueprint) Cache() error {
	// See if there was already a defined alias for this blueprint.
	if row := db.DB.QueryRow(`SELECT alias FROM blueprints WHERE type = ?`, b.DeviceType); row.Err() == nil {
		row.Scan(&b.Alias)
	}

	if b.Alias == "" || b.Alias == b.DeviceType {
		newAlias := cache.AliasType(b.DeviceType)
		if newAlias != b.DeviceType {
			b.Alias = newAlias
		}
	}
	if err := db.Update(cache.BlueprintsTable, map[string]any{
		"type":            b.DeviceType,
		"print_time":      b.PrintTime.seconds,
		"attach_capacity": b.AttachCapacity,
		"cargo_capacity":  b.CargoCapacity,
		"stow_capacity":   b.StowCapacity,
		"short":           b.ShortDescription,
		"description":     b.Description,
		"alias":           b.Alias,
	}); err != nil {
		return err
	}

	for r, q := range b.Resources {
		if err := db.Update(cache.BlueprintResTable, map[string]any{
			"blueprint_type": b.DeviceType,
			"type":           r,
			"qty":            q,
		}); err != nil {
			return err
		}
	}

	for r, q := range b.Components {
		if err := db.Update(cache.BlueprintCmpTable, map[string]any{
			"blueprint_type": b.DeviceType,
			"type":           r,
			"qty":            q,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (b *Blueprint) Get() error {
	if db == nil {
		return fmt.Errorf("Not connected to cache")
	}
	if b.DeviceType == "" {
		return fmt.Errorf("Can't load unknown blueprint")
	}
	scan, err := db.Get(cache.BlueprintsTable, b.DeviceType)
	if err != nil {
		return fmt.Errorf("Error querying cache: %v", err)
	}
	var pt float32
	err = scan(
		&b.DeviceType, &pt, &b.AttachCapacity, &b.CargoCapacity, &b.StowCapacity,
		&b.ShortDescription, &b.Description, &b.Alias)
	if err != nil {
		return err
	}
	d, err := time.ParseDuration(fmt.Sprintf("%fs", pt))
	if err != nil {
		return err
	}
	b.PrintTime = &JSONTimeDelta{pt, d}

	if b.Resources == nil {
		b.Resources = make(map[string]int)
	}
	rows, err := db.GetAll(cache.BlueprintResTable, b.DeviceType)
	if err != nil {
		return err
	}
	for rows.Next() {
		var t, r string
		var q int
		if err := rows.Scan(&t, &r, &q); err != nil {
			return err
		}
		b.Resources[r] = q
	}
	if b.Components == nil {
		b.Components = make(map[string]int)
	}
	rows, err = db.GetAll(cache.BlueprintCmpTable, b.DeviceType)
	if err != nil {
		return err
	}
	for rows.Next() {
		var t, r string
		var q int
		if err := rows.Scan(&t, &r, &q); err != nil {
			return err
		}
		b.Components[r] = q
	}
	return nil
}

type Blueprints struct {
	Blueprints []*Blueprint `json:"blueprints"`
}

func (bs *Blueprints) Cache() error {
	var errs []error
	for _, b := range bs.Blueprints {
		errs = append(errs, b.Cache())
	}
	return errors.Join(errs...)
}

func (bs *Blueprints) Get() error {
	if db == nil {
		return fmt.Errorf("Not connected to cache")
	}
	all, err := db.ListIDs(cache.BlueprintsTable)
	if err != nil {
		return err
	}
	for _, t := range cache.Strs(all) {
		bp := &Blueprint{DeviceType: t}
		if err := bp.Get(); err != nil {
			return err
		}
		bs.Blueprints = append(bs.Blueprints, bp)
	}
	return nil
}

type PrintResp struct {
	Status            string         `json:"status"`
	DeviceType        string         `json:"device_type"`
	Started           *JSONTime      `json:"started_at"`
	Completes         *JSONTime      `json:"completes_at"`
	PrintTime         *JSONTimeDelta `json:"print_time_seconds"`
	ResourcesRefunded bool           `json:"resources_refunded"`
}

func (pr *PrintResp) Notification() *Notification {
	if pr.Started == nil || pr.Completes == nil {
		return nil
	}
	return &Notification{
		Start: pr.Started.ts,
		End:   pr.Completes.ts,
		Text:  fmt.Sprintf("Finished printing %s", pr.DeviceType),
	}
}

type Queued struct {
	Queue       []string `json:"queue"`
	QueueLength int      `json:"queue_length"`
	Status      string   `json:"status"`
}
