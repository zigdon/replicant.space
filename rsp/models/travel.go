package models

import (
	"time"
)

type TripLeg struct {
	Active      bool    `json:"active"`
	DistanceAu  float32 `json:"distance_au"`
	From        string  `json:"from"`
	FromName    string  `json:"from_name"`
	Leg         int     `json:"leg"`
	Time        time.Duration
	TimeSeconds float32 `json:"time_seconds"`
	To          string  `json:"to"`
	ToName      string  `json:"to_name"`
	Type        string  `json:"type"`
}

func (tl *TripLeg) Fill() error {
	return fillDuration(tl.TimeSeconds, &tl.Time)
}

type Trip struct {
	Arrives          time.Time
	ArrivesAt        string `json:"arrives_at"`
	Departed         time.Time
	DepartedAt       string  `json:"departed_at"`
	Destination      string  `json:"destination"`
	DestinationName  string  `json:"destination_name"`
	DistanceLy       float32 `json:"distance_ly"`
	Error            string  `json:"error"`
	EtaSeconds       float32 `json:"eta_seconds"`
	Eta              time.Duration
	Origin           string    `json:"origin"`
	OriginName       string    `json:"origin_name"`
	ProgressPercent  float32   `json:"progress_percent"`
	Route            []TripLeg `json:"route"`
	Status           string    `json:"status"`
	TotalTime        time.Duration
	TotalTimeSeconds float32 `json:"total_time_seconds"`
	Type             string  `json:"type"`
}

func (t *Trip) Fill() error {
	if err := fillDuration(t.EtaSeconds, &t.Eta); err != nil {
		return err
	}
	if err := fillDuration(t.TotalTimeSeconds, &t.TotalTime); err != nil {
		return err
	}
	if err := fillTime(t.ArrivesAt, &t.Arrives); err != nil {
		return err
	}
	if err := fillTime(t.DepartedAt, &t.Departed); err != nil {
		return err
	}
	for _, l := range t.Route {
		if err := l.Fill(); err != nil {
			return err
		}
	}
	return nil
}

type JourneyLeg struct {
	From         string
	FromPosition Position
	To           string
	ToPosition   Position
	DistFromSrc  float32
	DistToDest   float32
	Processed    bool
}

type Journey struct {
	Source         string
	SourcePosition Position
	Dest           string
	DestPosition   Position
	Legs           []JourneyLeg
}
