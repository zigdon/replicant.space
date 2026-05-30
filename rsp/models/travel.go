package models

import (
	"fmt"

	"encoding/json"
)

type TripLegs struct {
	Leg int `yaml:"leg"`
	From string `yaml:"from"`
	To string `yaml:"to"`
	Type string `yaml:"type"`
	TimeSeconds int `yaml:"time_seconds"`
}

type Trip struct {
	Origin string `yaml:"origin"`
	Destination string `yaml:"destination"`
	DepartedAt string `yaml:"departed_at"`
	ArrivesAt string `yaml:"arrives_at"`
	TotalTimeSeconds float32 `yaml:"total_time_seconds"`
	Route []TripLegs `yaml:"route"`
	Status string `yaml:"status"`
}

func ParseTrip(data []byte) (*Trip, error) {
	s := &Trip{}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("Error parsing trip: %v", err)
	}

	return s, nil
}
