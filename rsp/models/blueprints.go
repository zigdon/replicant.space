package models

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/lib/pq"
	"github.com/zigdon/rsp/cache"
)

type Blueprint struct {
	AttachCapacity   int            `json:"attach_capacity"`
	CargoCapacity    int            `json:"cargo_capacity"`
	Components       map[string]int `json:"components"`
	DeviceType       string         `json:"device_type"`
	Description      string         `json:"description"`
	Directives       pq.StringArray `json:"directives"`
	Features         pq.StringArray `json:"features"`
	PrintTime        *JSONTimeDelta `json:"print_time"`
	Resources        map[string]int `json:"resources"`
	ShortDescription string         `json:"short_description"`
	StowCapacity     int            `json:"stow_capacity"`
	Strength         float32        `json:"strength"`
}

func (b *Blueprint) Cache() error {
	ing := make(map[string]int)
	maps.Copy(ing, b.Components)
	maps.Copy(ing, b.Resources)
	return db.Update(cache.BlueprintsTable, map[string]any{
		"type":            b.DeviceType,
		"print_time":      b.PrintTime.seconds,
		"attach_capacity": b.AttachCapacity,
		"cargo_capacity":  b.CargoCapacity,
		"stow_capacity":   b.StowCapacity,
		"short":           b.ShortDescription,
		"description":     b.Description,
		"directives":      b.Directives,
		"features":        b.Features,
		"ingredients":     cache.Encode(ing),
	})
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
	var pt string
	var ing cache.JSONB[map[string]int]
	err = scan(
		&b.DeviceType, &pt, &b.AttachCapacity, &b.CargoCapacity, &b.StowCapacity,
		&b.ShortDescription, &b.Description, &b.Features, &b.Directives, &ing)
	if err != nil {
		return err
	}
	d, err := psqlDuration(pt)
	if err != nil {
		return err
	}
	b.PrintTime = &JSONTimeDelta{float32(d.Seconds()), d}
	if b.Components == nil {
		b.Components = make(map[string]int)
	}
	if b.Resources == nil {
		b.Resources = make(map[string]int)
	}
	for k, v := range ing.Data {
		if slices.Contains(
			[]string{"carbon", "conductive", "rares", "silicates", "structural", "volatiles"}, k) {
			b.Resources[k] = v
		} else {
			b.Components[k] = v
		}
	}
	return nil
}

func (b *Blueprint) RawResources() (map[string]int, error) {
	res := make(map[string]int)
	maps.Copy(res, b.Resources)
	for c, n := range b.Components {
		cb := &Blueprint{DeviceType: c}
		if err := cb.Get(); err != nil {
			return nil, fmt.Errorf("Can't load blueprint %q: %v", c, err)
		}
		cRes, err := cb.RawResources()
		if err != nil {
			return nil, fmt.Errorf("Can't load resources for %q component: %v", c, err)
		}
		for k, v := range cRes {
			res[k] += v * n
		}
	}

	return res, nil
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
	bpm := make(map[string]*Blueprint)
	if rows, err := db.DB.Query(`
		SELECT type, print_time, attach_capacity, cargo_capacity, stow_capacity, short, description,
			   directives, features, ingredients
		FROM blueprints
	`); err != nil {
		return err
	} else {
		defer rows.Close()
		for rows.Next() {
			bp := &Blueprint{
				PrintTime:  new(JSONTimeDelta),
				Resources:  make(map[string]int),
				Components: make(map[string]int),
			}
			var pt string
			var ing cache.JSONB[map[string]int]
			if err := rows.Scan(
				&bp.DeviceType, &pt, &bp.AttachCapacity, &bp.CargoCapacity, &bp.StowCapacity,
				&bp.ShortDescription, &bp.Description, &bp.Directives, &bp.Features, &ing,
			); err != nil {
				return err
			}
			d, err := psqlDuration(pt)
			if err != nil {
				return err
			}
			bp.PrintTime.td = d
			for k, v := range ing.Data {
				if slices.Contains(
					[]string{"carbon", "conductive", "rares", "silicates", "structural", "volatiles"}, k) {
					bp.Resources[k] = v
				} else {
					bp.Components[k] = v
				}
			}
			bpm[bp.DeviceType] = bp
		}
		if err := rows.Err(); err != nil {
			return err
		}
	}
	for _, bp := range bpm {
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
		Start:  pr.Started.ts,
		End:    pr.Completes.ts,
		Text:   fmt.Sprintf("Finished printing %s", pr.DeviceType),
		Object: pr,
	}
}

type Queued struct {
	Queue       []string `json:"queue"`
	QueueLength int      `json:"queue_length"`
	Status      string   `json:"status"`
}
