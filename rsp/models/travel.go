package models

import (
	"time"
)

type TripLegs struct {
	Active      bool    `json:"active"`
	Distance_au float32 `json:"distance_au"`
	From        string  `json:"from"`
	FromName    string  `json:"from_name"`
	Leg         int     `json:"leg"`
	Time        time.Duration
	TimeSeconds float32 `json:"time_seconds"`
	To          string  `json:"to"`
	ToName      string  `json:"to_name"`
	Type        string  `json:"type"`
}

type Trip struct {
	ArrivesAt        string     `json:"arrives_at"`
	DepartedAt       string     `json:"departed_at"`
	Destination      string     `json:"destination"`
	DestinationName  string     `json:"destination_name"`
	DistanceLy       float32    `json:"distance_ly"`
	Origin           string     `json:"origin"`
	OriginName       string     `json:"origin_name"`
	ProgressPercent  float32    `json:"progress_percent"`
	Route            []TripLegs `json:"route"`
	Status           string     `json:"status"`
	TotalTime        time.Duration
	TotalTimeSeconds float32 `json:"total_time_seconds"`
	Type             string  `json:"type"`
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
