package models

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Resources struct {
	Carbon string `yaml:"carbon"`
	Conductive string `yaml:"conductive"`
	Rares string `yaml:"rares"`
	Silicates string `yaml:"silicates"`
	Structural string `yaml:"structural"`
	Volatiles string `yaml:"volatiles"`
}

type Belts struct {
	Density string `yaml:"density"`
	Designation string `yaml:"designation"`
	InnerRadiusAu float32 `yaml:"inner_radius_au"`
	OuterRadiusAu float32 `yaml:"outer_radius_au"`
	Resources Resources `yaml:"resources"`
}

type Destination struct {
	Designation string `yaml:"designation"`
	DistanceAu float32 `yaml:"distanceAu"`
}

type Salvage struct {
	Depleted bool `yaml:"depleted"`
	Designation string `yaml:"designation"`
	Location string `yaml:"location"`
	Name string `yaml:"name"`
	ResourcesAvailable []string `yaml:"resources_available"`
	SalvageType string `yaml:"salvage_type"`
}

type Planet struct {
	Designation string `yaml:"designation"`
	InHabitableZone bool `yaml:"in_habitable_zone"`
	MoonCount int `yaml:"moon_count"`
	Name string `yaml:"name"`
	OrbitalDistanceAu float32 `yaml:"orbital_distance_au"`
	Salvage []Salvage `yaml:"salvage"`
	Scanned bool `yaml:"scanned"`
	Type string `yaml:"type"`
}

type ScanReplicant struct {
	LastActive string `yaml:"last_active"`
	Location string `yaml:"location"`
	ReplicantCode string `yaml:"replicant_code"`
}

type Position struct {
	x float32
	y float32
	z float32
}

type Star struct {
	AgeMy float32 `yaml:"age_my"`
	Color string `yaml:"color"`
	Designation string `yaml:"designation"`
    habitable_zone struct {
		InnerAu float32 `yaml:"inner_au"`
		OuterAu float32 `yaml:"outer_au"`
	} `yaml:"habitable_zone"`
	LuminositySolar float32 `yaml:"luminositysolar"`
	MassSolar float32 `yaml:"mass_solar"`
	Name string `yaml:"name"`
	Position Position  `yaml:"position"`
	SpectralType string `yaml:"spectral_type"`
	TemperatureK int `yaml:"temperature_k"`
}

type Object struct {
	ActivePlates int `yaml:"active_plates"`
	CurrentProgress float32 `yaml:"current_progress"`
	Designation string `yaml:"designation"`
	ImpactEta string `yaml:"impact_eta"`
	ImpactTarget string `yaml:"impact_target"`
	ObjectType string `yaml:"object_type"`
	OrbitalDistanceAu float32 `yaml:"orbital_distance_au"`
	ProgressPct float32 `yaml:"progress_pct"`
	RequiredStrength float32 `yaml:"required_strength"`
	SizeClass string `yaml:"size_class"`
	Status string `yaml:"status"`
}

type Scan struct {
	ActiveLocationEvents []string `yaml:"active_location_events"`
	asteroid_belt struct {
		Belts []Belts `yaml:"belts"`
		Present bool `yaml:"present"`
	} `yaml:"asteroid_belt"`
	EntryPoint string `yaml:"entry_point"`
	LifeDetected bool `yaml:"life_detected"`
	MiningBonusPct int `yaml:"mining_bonus_pct"`
	outer_system struct {
		Kuiper Destination `yaml:"kuiper"`
		Oort Destination `yaml:"oort"`
	} `yaml:"outer_system"`
	Planets []Planet `yaml:"planets"`
	Replicants map[string]ScanReplicant `yaml:"replicants"`
	Star Star `yaml:"star"`
	SystemObjects []Object `yaml:"system_objects"`
	SystemTags []string `yaml:"system_tags"`
}

func ParseScan(data []byte) (*Scan, error) {
	s := &Scan{}
	if err := yaml.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("Error parsing scan: %v", err)
	}

	return s, nil
}
