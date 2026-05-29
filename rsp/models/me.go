package models

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Notify struct {
	Email bool `yaml:"email"`
	Preferences map[string]bool `yaml:"preferences"`
	Webhook bool `yaml:"webhook"`
}

type Replicant struct {
	CreatedAt string `yaml:"created_at"`
	CurrentLocation string `yaml:"current_location"`
	CurrentLocationName string `yaml:"current_location_name"`
	CurrentStar string `yaml:"current_star"`
	CurrentStarName string `yaml:"current_star_name"`
	DeviceCount int `yaml:"device_count"`
	ExperiencePoints int `yaml:"experience_points"`
	HostedDeviceCode string `yaml:"hosted_device_code"`
	Name string `yaml:"name"`
	ReplicantCode string `yaml:"replicant_code"`
}

type Me struct {
	BobnetChannels []string `yaml:"bobnet_channels"`
	CreatedAt string `yaml:"created_at"`
	Email string `yaml:"email"`
	EmailVerified bool `yaml:"email_verified"`
	ExperiencePointsTotal int `yaml:"experience_points_total"`
	MessageNotify Notify `yaml:"message_notify"`
	Name string `yaml:"name"`
	Replicants []Replicant `yaml:"replicants"`
	Status string `yaml:"status"`
	Timezone string `yaml:"timezone"`
	UnreadMessageCount int `yaml:"unread_message_count"`
}

func ParseMe(data []byte) (*Me, error) {
	m := &Me{}
	if err := yaml.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("Error parsing me: %v", err)
	}

	return m, nil
}
