package models

import (
	"fmt"

	"encoding/json"
)

type TripLegs struct {
	Leg int `json:"leg"`
	From string `json:"from"`
	To string `json:"to"`
	Type string `json:"type"`
	TimeSeconds int `json:"time_seconds"`
}

type Trip struct {
	Origin string `json:"origin"`
	Destination string `json:"destination"`
	DepartedAt string `json:"departed_at"`
	ArrivesAt string `json:"arrives_at"`
	TotalTimeSeconds float32 `json:"total_time_seconds"`
	Route []TripLegs `json:"route"`
	Status string `json:"status"`
}

func ParseTrip(data []byte) (*Trip, error) {
	s := &Trip{}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("Error parsing trip: %v", err)
	}

	return s, nil
}
