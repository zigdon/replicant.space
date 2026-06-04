package models

import (
	"time"
)

type TripLegs struct {
	Leg         int           `json:"leg"`
	From        string        `json:"from"`
	To          string        `json:"to"`
	Type        string        `json:"type"`
	TimeSeconds float32       `json:"time_seconds"`
	Time        time.Duration `json:"time_seconds"`
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
