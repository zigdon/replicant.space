package models

import (
	"fmt"

	"encoding/json"
)

type OwnedDevices struct {
	Devices []Device `json:"devices"`
}

type PrintQueue struct {
	DeviceType string `json:"device_type"`
	Notify struct {
		Device string `json:"device"`
		Email bool `json:"email"`
		Webhook bool `json:"webhook"`
	} `json:"notify"`
}

type MissingResources struct {
	Have int `json:"have"`
	Need int `json:"need"`
}

type Replicant struct {
	AttachedDevices []string `json:"attached_devices"`
	Cargo []string `json:"cargo"`
	CreatedAt string `json:"created_at"`
	CurrentLocation string `json:"current_location"`
	CurrentLocationName string `json:"current_location_name"`
	CurrentStar string `json:"current_star"`
	CurrentStarName string `json:"current_star_name"`
	Description string `json:"description"`
	DeviceCount int `json:"device_count"`
	ExperiencePoints int `json:"experience_points"`
	HostedDeviceCode string `json:"hosted_device_code"`
	Location string `json:"location"`
	LocationName string `json:"location_name"`
	Name string `json:"name"`
	Position Position `json:"position"`
	PrintQueue []PrintQueue `json:"print_queue"`
	ReplicantCode string `json:"replicant_code"`
	Status string `json:"status"`
	StowedDevices []Device `json:"stowed_devices"`
	WaitingFor map[string]MissingResources `json:"waiting_for"`
}

func (r *Replicant) GetDeviceIDs() []string {
	var res []string
	for _, d := range r.StowedDevices {
		res = append(res, d.Code)
	}
	return res
}

func ParseReplicant(data []byte) (*Replicant, error) {
	r := &Replicant{}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("Error parsing replicant: %v", err)
	}

	return r, nil
}

func ParseOwnedDevices(data []byte) ([]Device, error) {
	od := &OwnedDevices{}
	if err := json.Unmarshal(data, od); err != nil {
		return nil, fmt.Errorf("Error parsing device list: %v", err)
	}

	return od.Devices, nil
}


