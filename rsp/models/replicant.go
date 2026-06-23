package models

import (
	"fmt"
	"strings"
)

type ReplicantEvent struct {
	Created    *JSONTime      `json:"created_at"`
	DeviceCode *CodeAlias     `json:"device_code"`
	DeviceType string         `json:"device_type"`
	Type       string         `json:"event_type"`
	Message    string         `json:"message"`
	Payload    map[string]any `json:"payload"`
}

type ReplicantEvents struct {
	ReplicantEvents []*ReplicantEvent `json:"events"`
}

type OwnedDevices struct {
	Devices []*Device `json:"devices"`
}

type PrintQueue struct {
	DeviceType string `json:"device_type"`
	Notify     struct {
		Device  string `json:"device"`
		Email   bool   `json:"email"`
		Webhook bool   `json:"webhook"`
	} `json:"notify"`
}

type MissingResources struct {
	Have int `json:"have"`
	Need int `json:"need"`
}

type Travel struct {
	Arrives         *JSONTime      `json:"arrives_at"`
	Departed        *JSONTime      `json:"departed_at"`
	Destination     string         `json:"destination"`
	DestinationName string         `json:"destination_name"`
	DestinationType string         `json:"destination_type"`
	Eta             *JSONTimeDelta `json:"eta_seconds"`
	Origin          string         `json:"origin"`
	OriginName      string         `json:"origin_name"`
	ProgressPercent float32        `json:"progress_percent"`
	Route           []*TripLeg     `json:"route"`
	Stage           string         `json:"stage"`
	TotalDistanceLy float32        `json:"total_distance_ly"`
	TotalTime       *JSONTimeDelta `json:"total_time_seconds"`
	Type            string         `json:"type"`
}

type Replicant struct {
	AttachedDevices     []string                     `json:"attached_devices"`
	Cargo               []string                     `json:"cargo"`
	Created             *JSONTime                    `json:"created_at"`
	CurrentLocation     string                       `json:"current_location"`
	CurrentLocationName string                       `json:"current_location_name"`
	CurrentStar         string                       `json:"current_star"`
	CurrentStarName     string                       `json:"current_star_name"`
	Description         string                       `json:"description"`
	DeviceCount         int                          `json:"device_count"`
	ExperiencePoints    int                          `json:"experience_points"`
	HostedDeviceCode    *CodeAlias                   `json:"hosted_device_code"`
	IsNPC               bool                         `json:"is_npc"`
	Location            string                       `json:"location"`
	LocationName        string                       `json:"location_name"`
	Name                string                       `json:"name"`
	Plan                string                       `json:"plan"`
	Position            *Position                    `json:"position"`
	PrintQueue          []*PrintQueue                `json:"print_queue"`
	Project             string                       `json:"project"`
	Pronouns            string                       `json:"pronouns"`
	ReplicantCode       *CodeAlias                   `json:"replicant_code"`
	Status              string                       `json:"status"`
	StowedDevices       []*Device                    `json:"stowed_devices"`
	Travel              *Travel                      `json:"travel"`
	WaitingFor          map[string]*MissingResources `json:"waiting_for"`
}

func (r *Replicant) Details() *Grid {
	is := []GridItem{
			{Title: r.ReplicantCode.Alias(), Text: r.Name, W: 2},
			{Y: 1, Title: "Location", Text: r.CurrentLocation},
			{Y: 1, X: 1, Title: "Status", Text: r.Status},
		}
	var l []string
	for _, pq := range r.PrintQueue {
		l = append(l, pq.DeviceType)
	}
	is = append(is, GridItem{Y: 2, Title: "Print Queue", Text: strings.Join(l, "\n")})
	l = []string{}
	for _, d := range r.StowedDevices {
		l = append(l, d.Type)
	}
	is = append(is, GridItem{Y: 2, X:1, Title: "Stowed", Text: strings.Join(l, "\n")})

	return &Grid{Items: is}
}

func (r *Replicant) GetDeviceIDs() []string {
	var res []string
	for _, d := range r.StowedDevices {
		res = append(res, d.Code.String())
	}
	return res
}

func (r *Replicant) ListItem() (string, string) {
	return fmt.Sprintf("%s: %s", r.ReplicantCode.Alias(), r.Name), r.CurrentLocation
}

