package models

import "fmt"

type Position struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

func (p Position) String() string {
	return fmt.Sprintf("[%.2f/%.2f/%.2f]", p.X, p.Y, p.Z)
}

type Star struct {
	AgeMy float32 `json:"age_my"`
	Color string `json:"color"`
	Designation string `json:"designation"`
	DistanceFromReplicant float32 `json:"distance_from_replicant"`
	EntryPoint string `json:"entry_point"`
	EstimatedPlanets int `json:"estimated_planets"`
	EstimatedTravelTime int `json:"estimated_travel_time"`
	Explored bool `json:"explored"`
    HabitableZone struct {
		InnerAu float32 `json:"inner_au"`
		OuterAu float32 `json:"outer_au"`
	} `json:"habitable_zone"`
	HasLife bool `json:"has_life"`
	LuminositySolar float32 `json:"luminositysolar"`
	MassSolar float32 `json:"mass_solar"`
	Name string `json:"name"`
	Position Position  `json:"position"`
	SpectralType string `json:"spectral_type"`
	TemperatureK int `json:"temperature_k"`
}

type Census struct {
	Page int `json:"page"`
	PerPage int `json:"per_page"`
	ReplicantPosition Position `json:"replicant_position"`
	Stars []Star `json:"stars"`
	Total int `json:"total"`
	TotalPages int `json:"total_pages"`
	TotalStars int `json:"total_stars"`
}

type Belt struct {
	Density string `json:"density"`
	Designation string `json:"designation"`
	InnerRadiusAu float32 `json:"inner_radius_au"`
	OuterRadiusAu float32 `json:"outer_radius_au"`
	Resources map[string]string `json:"resources"`
}

type Inventory struct {
	Quantity float32 `json:"quantity"`
	ResourceType string `json:"resource_type"`
}

type Site struct {
	Designation string `json:"designation"`
	Name string `json:"name"`
	ResourcesRemaining_pct map[string]int `json:"resources_remaining_pct"`
	SiteIndex int `json:"site_index"`
}

type Location struct {
	Belt Belt `json:"belt"`
	Devices []Device `json:"devices"`
	Inventory []Inventory `json:"inventory"`
	Location string `json:"location"`
	LocationType string `json:"location_type"`
	ResourceSites []Site `json:"resource_sites"`
}
