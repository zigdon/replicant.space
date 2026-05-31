package models

import (
	"fmt"
	"sort"

	"encoding/json"
)

type Destination struct {
	Designation string `json:"designation"`
	DistanceAu float32 `json:"distanceAu"`
}

type Salvage struct {
	Depleted bool `json:"depleted"`
	Designation string `json:"designation"`
	Location string `json:"location"`
	Name string `json:"name"`
	ResourcesAvailable []string `json:"resources_available"`
	SalvageType string `json:"salvage_type"`
}

type Planet struct {
	Designation string `json:"designation"`
	InHabitableZone bool `json:"in_habitable_zone"`
	MoonCount int `json:"moon_count"`
	Name string `json:"name"`
	OrbitalDistanceAu float32 `json:"orbital_distance_au"`
	Salvage []Salvage `json:"salvage"`
	Scanned bool `json:"scanned"`
	Type string `json:"type"`
}

type ScanReplicant struct {
	LastActive string `json:"last_active"`
	Location string `json:"location"`
	ReplicantCode string `json:"replicant_code"`
}

type Position struct {
	x float32
	y float32
	z float32
}

type Star struct {
	AgeMy float32 `json:"age_my"`
	Color string `json:"color"`
	Designation string `json:"designation"`
    HabitableZone struct {
		InnerAu float32 `json:"inner_au"`
		OuterAu float32 `json:"outer_au"`
	} `json:"habitable_zone"`
	LuminositySolar float32 `json:"luminositysolar"`
	MassSolar float32 `json:"mass_solar"`
	Name string `json:"name"`
	Position Position  `json:"position"`
	SpectralType string `json:"spectral_type"`
	TemperatureK int `json:"temperature_k"`
}

type Object struct {
	ActivePlates int `json:"active_plates"`
	CurrentProgress float32 `json:"current_progress"`
	Designation string `json:"designation"`
	ImpactEta string `json:"impact_eta"`
	ImpactTarget string `json:"impact_target"`
	ObjectType string `json:"object_type"`
	OrbitalDistanceAu float32 `json:"orbital_distance_au"`
	ProgressPct float32 `json:"progress_pct"`
	RequiredStrength float32 `json:"required_strength"`
	SizeClass string `json:"size_class"`
	Status string `json:"status"`
}

type Scan struct {
	ActiveLocationEvents []string `json:"active_location_events"`
	AsteroidBelt struct {
		Belts []Belt `json:"belts"`
		Present bool `json:"present"`
	} `json:"asteroid_belt"`
	EntryPoint string `json:"entry_point"`
	LifeDetected bool `json:"life_detected"`
	MiningBonusPct int `json:"mining_bonus_pct"`
	OuterSystem struct {
		Kuiper Destination `json:"kuiper"`
		Oort Destination `json:"oort"`
	} `json:"outer_system"`
	Planets []Planet `json:"planets"`
	Replicants map[string]ScanReplicant `json:"replicants"`
	Star Star `json:"star"`
	SystemObjects []Object `json:"system_objects"`
	SystemTags []string `json:"system_tags"`
}

func (s *Scan) ExtractLocations() []string {
	if s == nil { return nil }
	locs := make(map[string]bool)
	a := func(loc string) { locs[loc] = true }
	if s.EntryPoint != "" { a(s.EntryPoint) }
	a(s.Star.Designation)
	for _, b := range s.AsteroidBelt.Belts { a(b.Designation) }
	if s.OuterSystem.Oort.Designation != "" { a(s.OuterSystem.Oort.Designation) }
	if s.OuterSystem.Kuiper.Designation != "" { a(s.OuterSystem.Kuiper.Designation) }
	for _, p := range s.Planets { a(p.Designation) }
	for _, v := range s.Replicants { a(v.Location) }
	for _, o := range s.SystemObjects { a(o.Designation) }

	// Dedup and sort before returning.
	var res []string
	for l := range locs { res = append(res, l) }

	return sort.StringSlice(res)
}

func ParseScan(data []byte) (*Scan, error) {
	s := &Scan{}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("Error parsing scan: %v", err)
	}

	return s, nil
}
