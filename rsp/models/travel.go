package models

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/zigdon/rsp/cache"
)

type TripLeg struct {
	Active     bool           `json:"active"`
	DistanceAu float32        `json:"distance_au"`
	DistanceLy float32        `json:"distance_ly"`
	From       string         `json:"from"`
	FromName   string         `json:"from_name"`
	Leg        int            `json:"leg"`
	Time       *JSONTimeDelta `json:"time_seconds"`
	To         string         `json:"to"`
	ToName     string         `json:"to_name"`
	Type       string         `json:"type"`
}

type Trip struct {
	Arrives         *JSONTime      `json:"arrives_at"`
	Departed        *JSONTime      `json:"departed_at"`
	Destination     string         `json:"destination"`
	DestinationName string         `json:"destination_name"`
	DistanceLy      float32        `json:"distance_ly"`
	Error           string         `json:"error"`
	Eta             *JSONTimeDelta `json:"eta_seconds"`
	Origin          string         `json:"origin"`
	OriginName      string         `json:"origin_name"`
	ProgressPercent float32        `json:"progress_percent"`
	Route           []*TripLeg     `json:"route"`
	Status          string         `json:"status"`
	TotalTime       *JSONTimeDelta `json:"total_time_seconds"`
	Type            string         `json:"type"`

	Device *CodeAlias
}

func (t *Trip) Notification() *Notification {
	d := "Unknown device"
	if t.Device != nil {
		d = t.Device.Alias()
	}
	if t.Departed == nil || t.Arrives == nil {
		return nil
	}
	return &Notification{
		Start:  t.Departed.ts,
		End:    t.Arrives.ts,
		Device: d,
		Text:   fmt.Sprintf("Arrived at %s", t.Destination),
	}
}
func (t *Trip) Short() string {
	if t == nil {
		return ""
	}
	return fmt.Sprintf("%s -> %s (%.0f%%, %s)",
		t.Origin, t.Destination, t.ProgressPercent, t.Eta.String())
}

type JourneyLeg struct {
	JourneyID    int
	From         string
	FromPosition *Position
	To           string
	ToPosition   *Position
	DistFromSrc  float32
	DistToDest   float32
	Processed    bool
	Step         int
}

func (jl *JourneyLeg) Cache() error {
	return db.Update(cache.JourneyStepsTable, map[string]any{
		"journey_id": jl.JourneyID,
		"src":        jl.From,
		"dest":       jl.To,
		"dist_src":   jl.DistFromSrc,
		"dist_dest":  jl.DistToDest,
		"step":       jl.Step,
	})
}

func (jl *JourneyLeg) Get() error {
	// Implemented in parent
	return nil
}

func (jl *JourneyLeg) String() string {
	return fmt.Sprintf("%s->%s (%.2fly behind, %.2fly ahead)",
		jl.From, jl.To, jl.DistFromSrc, jl.DistToDest)
}

type Journey struct {
	ID             int
	Source         string
	SourcePosition *Position
	Dest           string
	DestPosition   *Position
	Legs           []*JourneyLeg
	MaxHop         float32
	Calculated     time.Time
}

func (j *Journey) ClearCache() error {
	if _, err := db.DB.Exec("DELETE FROM cached_journey_steps WHERE journey_id = ?", j.ID); err != nil {
		return fmt.Errorf("Can't delete old steps: %v", err)
	}
	if _, err := db.DB.Exec("DELETE FROM cached_journey WHERE id = ?", j.ID); err != nil {
		return fmt.Errorf("Can't delete old journey: %v", err)
	}
	return nil
}

func (j *Journey) Cache() error {
	// See if we already have an id for this journey
	row := db.DB.QueryRow(`SELECT id FROM cached_journey WHERE start = ? AND end = ?`, j.Source, j.Dest)
	if row.Err() == nil {
		if err := row.Scan(&j.ID); err == nil {
			log("Loaded existing ID: %d", j.ID)
		} else {
			// Find the next ID
			row := db.DB.QueryRow(`SELECT max(id)+1 from cached_journey`)
			if row.Err() != nil {
				return row.Err()
			}
			if err := row.Scan(&j.ID); err == nil {
				log("Setting new ID: %d", j.ID)
			}
		}
	}

	if j.Calculated.IsZero() {
		j.Calculated = time.Now()
	}
	if err := j.ClearCache(); err != nil {
		return fmt.Errorf("Can't clear old journey: %v", err)
	}

	if err := db.Update(cache.JourneyTable, map[string]any{
		"id":         j.ID,
		"start":      j.Source,
		"end":        j.Dest,
		"max_hop":    j.MaxHop,
		"calculated": j.Calculated.Unix(),
	}); err != nil {
		return fmt.Errorf("Error caching journey: %v", err)
	}

	// Reload so we get the auto-assigned ID. Save the existing legs so we can
	// cache them.
	legs := make([]*JourneyLeg, len(j.Legs))
	copy(legs, j.Legs)
	if j.ID == 0 {
		if err := j.Get(); err != nil {
			return fmt.Errorf("Error reloading journey: %v", err)
		}
	}

	var errs []error
	for i, jl := range legs {
		jl.JourneyID = j.ID
		if jl.From == "" && i > 0 {
			jl.From = legs[i-1].To
		}
		errs = append(errs, jl.Cache())
	}

	return errors.Join(errs...)
}

func (j *Journey) Get() error {
	if db == nil {
		return fmt.Errorf("Not connected to cache")
	}
	if j.Source == "" || j.Dest == "" {
		return fmt.Errorf("Can't load unknown journey")
	}

	row := db.DB.QueryRow(`
		SELECT id, start, end, max_hop, calculated
		FROM cached_journey
		WHERE start == ? AND end == ?
	`, j.Source, j.Dest)
	if err := row.Err(); err != nil {
		return err
	}
	var ts int64
	if err := row.Scan(&j.ID, &j.Source, &j.Dest, &j.MaxHop, &ts); err != nil {
		return err
	}

	j.Calculated = time.Unix(ts, 0)

	rows, err := db.DB.Query(`
        SELECT src, dest, dist_src, dist_dest, step
        FROM cached_journey_steps
        WHERE journey_id = ?
    `, j.ID)
	if err != nil {
		return err
	}
	var legs []*JourneyLeg
	for rows.Next() {
		jl := &JourneyLeg{JourneyID: j.ID}
		rows.Scan(&jl.From, &jl.To, &jl.DistFromSrc, &jl.DistToDest, &jl.Step)
		legs = append(legs, jl)
	}
	slices.SortFunc(legs, func(a, b *JourneyLeg) int {
		return cmp.Compare(a.Step, b.Step)
	})
	j.Legs = legs

	return nil
}
