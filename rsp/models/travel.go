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
	Origin           string  `json:"origin"`
	Destination      string  `json:"destination"`
	DepartedAt       string  `json:"departed_at"`
	ArrivesAt        string  `json:"arrives_at"`
	TotalTimeSeconds float32 `json:"total_time_seconds"`
	TotalTime        time.Duration
	Route            []TripLegs `json:"route"`
	Status           string     `json:"status"`
}
