package models

import (
	"fmt"

	"encoding/json"
)

type Notify struct {
	Email bool `json:"email"`
	Preferences map[string]bool `json:"preferences"`
	Webhook bool `json:"webhook"`
}

type Replicant struct {
	CreatedAt string `json:"created_at"`
	CurrentLocation string `json:"current_location"`
	CurrentLocationName string `json:"current_location_name"`
	CurrentStar string `json:"current_star"`
	CurrentStarName string `json:"current_star_name"`
	DeviceCount int `json:"device_count"`
	ExperiencePoints int `json:"experience_points"`
	HostedDeviceCode string `json:"hosted_device_code"`
	Name string `json:"name"`
	ReplicantCode string `json:"replicant_code"`
}

type Me struct {
	BobnetChannels []string `json:"bobnet_channels"`
	CreatedAt string `json:"created_at"`
	Email string `json:"email"`
	EmailVerified bool `json:"email_verified"`
	ExperiencePointsTotal int `json:"experience_points_total"`
	MessageNotify Notify `json:"message_notify"`
	Name string `json:"name"`
	Replicants []Replicant `json:"replicants"`
	Status string `json:"status"`
	Timezone string `json:"timezone"`
	UnreadMessageCount int `json:"unread_message_count"`
}

func ParseMe(data []byte) (*Me, error) {
	m := &Me{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("Error parsing me: %v", err)
	}

	return m, nil
}
