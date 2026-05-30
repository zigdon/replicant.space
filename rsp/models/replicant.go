package models

import (
	"fmt"

	"encoding/json"
)

type Device struct {
	Code string `json:"device_code"`
	Type string `json:"device_type"`
}

type Replicant struct {
	AttachedDevices []string `json:"attached_devices"`
	Cargo []string `json:"cargo"`
	CreatedAt string `json:"created_at"`
	CurrentLocation string `json:"current_location"`
	CurrentLocationName string `json:"current_location_name"`
	CurrentStar string `json:"current_star"`
	CurrentStarName string `json:"current_star_name"`
	DeviceCount int `json:"device_count"`
	ExperiencePoints int `json:"experience_points"`
	HostedDeviceCode string `json:"hosted_device_code"`
	Location string `json:"location"`
	LocationName string `json:"location_name"`
	Name string `json:"name"`
	Position Position `json:"position"`
	PrintQueue []string `json:"print_queue"`
	ReplicantCode string `json:"replicant_code"`
	Status string `json:"status"`
	StowedDevices []Device `json:"stowed_devices"`
}

func ParseReplicant(data []byte) (*Replicant, error) {
	r := &Replicant{}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("Error parsing replicant: %v", err)
	}

	return r, nil
}

