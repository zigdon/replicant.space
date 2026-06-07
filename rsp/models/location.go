package models

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Position struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

func NewPosition(x, y, z float32) Position {
	return Position{X: x, Y: y, Z: z}
}

func ParsePosition(coords string) (Position, error) {
	cs := strings.Split(coords, ",")
	p := Position{}
	if len(cs) != 3 {
		return p, fmt.Errorf("Destination must be specified as x,y,z")
	}
	x, err := strconv.Atoi(cs[0])
	if err != nil {
		return p, err
	}
	y, err := strconv.Atoi(cs[1])
	if err != nil {
		return p, err
	}
	z, err := strconv.Atoi(cs[2])
	if err != nil {
		return p, err
	}
	p.X = float32(x)
	p.Y = float32(y)
	p.Z = float32(z)
	return p, nil
}

func (p Position) Distance(to Position) float32 {
	return float32(math.Sqrt(
		math.Pow(float64(p.X-to.X), 2) +
			math.Pow(float64(p.Y-to.Y), 2) +
			math.Pow(float64(p.Z-to.Z), 2)))
}

func (p Position) String() string {
	return fmt.Sprintf("[%.2f/%.2f/%.2f]", p.X, p.Y, p.Z)
}

type Star struct {
	AgeMy                 float32 `json:"age_my"`
	Color                 string  `json:"color"`
	Designation           string  `json:"designation"`
	DistanceFromReplicant float32 `json:"distance_from_replicant"`
	DistanceFromSol       float32 `json:"distance_from_sol"`
	EntryPoint            string  `json:"entry_point"`
	EstimatedPlanets      int     `json:"estimated_planets"`
	EstimatedTravelTime   int     `json:"estimated_travel_time"`
	Explored              bool    `json:"explored"`
	HabitableZone         struct {
		InnerAu float32 `json:"inner_au"`
		OuterAu float32 `json:"outer_au"`
	} `json:"habitable_zone"`
	HasLife         bool     `json:"has_life"`
	LuminositySolar float32  `json:"luminositysolar"`
	MassSolar       float32  `json:"mass_solar"`
	MiningBonusPct  int      `json:"mining_bonus_pct"`
	Name            string   `json:"name"`
	Position        Position `json:"position"`
	SpectralType    string   `json:"spectral_type"`
	StellarClass    string   `json:"stellar_class"`
	TemperatureK    int      `json:"temperature_k"`
}

type Census struct {
	Page              int      `json:"page"`
	PerPage           int      `json:"per_page"`
	ReplicantPosition Position `json:"replicant_position"`
	Stars             []Star   `json:"stars"`
	Total             int      `json:"total"`
	TotalPages        int      `json:"total_pages"`
	TotalStars        int      `json:"total_stars"`
}

type Belt struct {
	Density       string            `json:"density"`
	Designation   string            `json:"designation"`
	InnerRadiusAu float32           `json:"inner_radius_au"`
	OuterRadiusAu float32           `json:"outer_radius_au"`
	Resources     map[string]string `json:"resources"`
}

type Inventory struct {
	Quantity     float32 `json:"quantity"`
	ResourceType string  `json:"resource_type"`
}

type Site struct {
	Designation           string         `json:"designation"`
	Index                 int            `json:"site_index"`
	Name                  string         `json:"name"`
	ResourcesRemainingPct map[string]int `json:"resources_remaining_pct"`
	Type                  string         `json:"site_type"`
}

type Planet struct {
	Atmosphere          string      `json:"atmosphere"`
	AxialTiltDeg        float32     `json:"axial_tilt_deg"`
	DensityGcc          float32     `json:"density_gcc"`
	Designation         string      `json:"designation"`
	InHabitableZone     bool        `json:"in_habitable_zone"`
	Inventory           []Inventory `json:"inventory"`
	LifeStage           string      `json:"life_stage"`
	LocationType        string      `json:"location_type"`
	MagneticField       bool        `json:"magnetic_field"`
	MassEarth           float32     `json:"mass_earth"`
	MoonCount           int         `json:"moon_count"`
	Name                string      `json:"name"`
	OrbitalDistanceAu   float32     `json:"orbital_distance_au"`
	OrbitalPeriodDays   float32     `json:"orbital_period_days"`
	RadiusEarth         float32     `json:"radius_earth"`
	Rings               bool        `json:"rings"`
	RotationPeriodHours float32     `json:"rotation_period_hours"`
	Salvage             []Salvage   `json:"salvage"`
	Scanned             bool        `json:"scanned"`
	SurfaceGravity      float32     `json:"surface_gravity"`
	SurfaceTempC        int         `json:"surface_temp_c"`
	SurfaceTempK        int         `json:"surface_temp_k"`
	Tags                []string    `json:"tags"`
	Type                string      `json:"type"`
}

type Moon struct {
	Designation  string `json:"designation"`
	Name         string `json:"name"`
	ParentPlanet string `json:"parent_planet"`
	Scanned      bool   `json:"scanned"`
	Type         string `json:"location_type"`
}

type LocationSummary struct {
	Devices        int `json:"devices"`
	LocationEvents int `json:"location_events"`
	Replicants     int `json:"replicants"`
	ResourceSites  int `json:"resource_sites"`
	Resources      int `json:"resources"`
}

type Location struct {
	AsteroidBelt struct {
		Belts []Belt `json:"belts"`
		Present bool `json:"present"`
	} `json:"asteroid_belt"`
	Belt                Belt                       `json:"belt"`
	Devices             []Device                   `json:"devices"`
	EntryPoint          string                     `json:"entry_point"`
	Inventory           []Inventory                `json:"inventory"`
	Location            string                     `json:"location"`
	Locations           map[string]LocationSummary `json:"locations"`
	Moon                Moon                       `json:"moon"`
	Moons               []Moon                     `json:"moons"`
	MoonsScanned        int                        `json:"moons_scanned"`
	MoonsTotal          int                        `json:"moons_total"`
	MoonsTotalEstimated bool                       `json:"moons_total_estimated"`
	Planet              Planet                     `json:"planet"`
	Planets             []Planet                   `json:"planets"`
	PlanetsScanned      int                        `json:"planets_scanned"`
	PlanetsTotal        int                        `json:"planets_total"`
	ResourceSites       []Site                     `json:"resource_sites"`
	Star                Star                       `json:"star"`
	SystemScanned       bool                       `json:"system_scanned"`
	Type                string                     `json:"location_type"`
}
