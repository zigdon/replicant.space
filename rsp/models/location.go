package models

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/zigdon/rsp/cache"
)

type Position struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

func NewPosition(x, y, z float32) *Position {
	return &Position{X: x, Y: y, Z: z}
}

func ParsePosition(coords string) (*Position, error) {
	var cs []string
	if strings.Contains(coords, ",") {
		cs = strings.Split(coords, ",")
	} else if strings.Contains(coords, ".") {
		cs = strings.Split(coords, ".")
	}
	if len(cs) != 3 {
		return nil, fmt.Errorf("Destination must be specified as x,y,z, got %q", coords)
	}
	x, err := strconv.Atoi(cs[0])
	if err != nil {
		return nil, err
	}
	y, err := strconv.Atoi(cs[1])
	if err != nil {
		return nil, err
	}
	z, err := strconv.Atoi(cs[2])
	if err != nil {
		return nil, err
	}
	p := &Position{
		X: float32(x),
		Y: float32(y),
		Z: float32(z),
	}
	return p, nil
}

func (p *Position) Distance(to *Position) float32 {
	if to == nil {
		return 0
	}
	return float32(math.Sqrt(
		math.Pow(float64(p.X-to.X), 2) +
			math.Pow(float64(p.Y-to.Y), 2) +
			math.Pow(float64(p.Z-to.Z), 2)))
}

func (p *Position) String() string {
	return fmt.Sprintf("[%.2f/%.2f/%.2f]", p.X, p.Y, p.Z)
}

func (p *Position) Reverse() {
	p.X *= -1
	p.Y *= -1
	p.Z *= -1
}

func (p *Position) Delta(pb *Position) *Position {
	return NewPosition(p.X-pb.X, p.Y-pb.Y, p.Z-pb.Z)
}

type StarCatalog struct {
	Total        int       `json:"total"`
	Generated_at *JSONTime `json:"generated_at"`
	Stars        []*Star   `json:"stars"`
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
	HasHub          bool      `json:"has_hub"`
	HasLife         bool      `json:"has_life"`
	LuminositySolar float32   `json:"luminositysolar"`
	MassSolar       float32   `json:"mass_solar"`
	MiningBonusPct  int       `json:"mining_bonus_pct"`
	Name            string    `json:"name"`
	Position        *Position `json:"position"`
	SpectralType    string    `json:"spectral_type"`
	StellarClass    string    `json:"stellar_class"`
	TemperatureK    int       `json:"temperature_k"`
}

func (s *Star) Cache() error {
	cur := &Star{Designation: s.Designation}
	if err := cur.Get(); err == nil {
		if cur.EstimatedPlanets > 0 && cur.EstimatedPlanets != s.EstimatedPlanets {
			s.EstimatedPlanets = cur.EstimatedPlanets
		}
		s.Explored = cur.Explored || s.Explored
	}
	return db.Update(cache.StarsTable, map[string]any{
		"designation": s.Designation,
		"entry_point": s.EntryPoint,
		"est_planets": s.EstimatedPlanets,
		"explored":    s.Explored,
		"has_life":    s.HasLife,
		"has_hub":     s.HasHub,
		"name":        s.Name,
		"position_x":  s.Position.X,
		"position_y":  s.Position.Y,
		"position_z":  s.Position.Z,
	})
}

func (s *Star) Get() error {
	if db == nil {
		return fmt.Errorf("Not connected to cache")
	}
	if s.Designation == "" {
		return fmt.Errorf("Can't load unknown star")
	}
	scan, err := db.Get(cache.StarsTable, s.Designation)
	if err != nil {
		return fmt.Errorf("Error querying cache: %v", err)
	}
	if s.Position == nil {
		p := NewPosition(0, 0, 0)
		s.Position = p
	}
	return scan(&s.Designation, &s.Name, &s.EntryPoint, &s.EstimatedPlanets, &s.Explored,
		&s.HasLife, &s.Position.X, &s.Position.Y, &s.Position.Z)
}

type Census struct {
	Page              int       `json:"page"`
	PerPage           int       `json:"per_page"`
	ReplicantPosition *Position `json:"replicant_position"`
	Stars             []*Star   `json:"stars"`
	Total             int       `json:"total"`
	TotalPages        int       `json:"total_pages"`
	TotalStars        int       `json:"total_stars"`
}

type Belt struct {
	Density       string            `json:"density"`
	Designation   string            `json:"designation"`
	InnerRadiusAu float32           `json:"inner_radius_au"`
	OuterRadiusAu float32           `json:"outer_radius_au"`
	Resources     map[string]string `json:"resources"`
	Star          string
}

func (b *Belt) Cache() error {
	var errs []error
	errs = append(errs, db.Update(cache.BeltsTable, map[string]any{
		"designation": b.Designation,
		"star":        b.Star,
		"density":     b.Density,
	}))
	for k, v := range b.Resources {
		errs = append(errs, db.Update(cache.BeltResTable, map[string]any{
			"belt":     b.Designation,
			"resource": k,
			"density":  v,
		}))
	}

	return errors.Join(errs...)
}

func (b *Belt) Get() error {
	var errs []error
	if db == nil {
		return fmt.Errorf("Not connected to cache")
	}
	if b.Designation == "" {
		return fmt.Errorf("Can't load unknown belt")
	}
	scan, err := db.Get(cache.BeltsTable, b.Designation)
	if err != nil {
		return fmt.Errorf("Error querying cache: %v", err)
	}
	errs = append(errs, scan(&b.Designation, &b.Star, &b.Density))

	// TODO load resources from cache.

	return errors.Join(errs...)
}

type Site struct {
	Designation           string         `json:"designation"`
	Index                 int            `json:"site_index"`
	Name                  string         `json:"name"`
	ResourcesRemainingPct map[string]int `json:"resources_remaining_pct"`
	Type                  string         `json:"site_type"`
}

type Planet struct {
	Atmosphere          string       `json:"atmosphere"`
	AxialTiltDeg        float32      `json:"axial_tilt_deg"`
	DensityGcc          float32      `json:"density_gcc"`
	Designation         string       `json:"designation"`
	InHabitableZone     bool         `json:"in_habitable_zone"`
	Inventory           []*Inventory `json:"inventory"`
	LifeStage           string       `json:"life_stage"`
	LocationType        string       `json:"location_type"`
	MagneticField       bool         `json:"magnetic_field"`
	MassEarth           float32      `json:"mass_earth"`
	MoonCount           int          `json:"moon_count"`
	Name                string       `json:"name"`
	OrbitalDistanceAu   float32      `json:"orbital_distance_au"`
	OrbitalPeriodDays   float32      `json:"orbital_period_days"`
	RadiusEarth         float32      `json:"radius_earth"`
	Rings               bool         `json:"rings"`
	RotationPeriodHours float32      `json:"rotation_period_hours"`
	Salvage             []*Salvage   `json:"salvage"`
	Scanned             bool         `json:"scanned"`
	SurfaceGravity      float32      `json:"surface_gravity"`
	SurfaceTempC        int          `json:"surface_temp_c"`
	SurfaceTempK        int          `json:"surface_temp_k"`
	Tags                []string     `json:"tags"`
	Type                string       `json:"type"`
	Star                string
}

func (p *Planet) Cache() error {
	return db.Update(cache.PlanetsTable, map[string]any{
		"designation": p.Designation,
		"star":        p.Star,
		"name":        p.Name,
		"life_stage":  p.LifeStage,
		"moons":       p.MoonCount,
		"rings":       p.Rings,
		"scanned":     p.Scanned,
	})
}

func (p *Planet) Get() error {
	if db == nil {
		return fmt.Errorf("Not connected to cache")
	}
	if p.Designation == "" {
		return fmt.Errorf("Can't load unknown planet")
	}
	scan, err := db.Get(cache.PlanetsTable, p.Designation)
	if err != nil {
		return fmt.Errorf("Error querying cache: %v", err)
	}
	return scan(&p.Designation, &p.Star, &p.Name, &p.LifeStage, &p.MoonCount,
		&p.Rings, &p.Scanned)
}

type Moon struct {
	Designation  string `json:"designation"`
	Name         string `json:"name"`
	ParentPlanet string `json:"parent_planet"`
	Star         string
	Scanned      bool   `json:"scanned"`
	Type         string `json:"location_type"`
}

func (m *Moon) Cache() error {
	return db.Update(cache.MoonsTable, map[string]any{
		"designation": m.Designation,
		"name":        m.Name,
		"planet":      m.ParentPlanet,
		"star":        m.Star,
		"scanned":     m.Scanned,
		"type":        m.Type,
	})
}

func (m *Moon) Get() error {
	if db == nil {
		return fmt.Errorf("Not connected to cache")
	}
	if m.Designation == "" {
		return fmt.Errorf("Can't load unknown moon")
	}
	scan, err := db.Get(cache.MoonsTable, m.Designation)
	if err != nil {
		return fmt.Errorf("Error querying cache: %v", err)
	}
	return scan(&m.Designation, &m.ParentPlanet, &m.Star, &m.Name, &m.Scanned, &m.Type)
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
		Belts   []Belt `json:"belts"`
		Present bool   `json:"present"`
	} `json:"asteroid_belt"`
	Belt                *Belt                       `json:"belt"`
	Devices             []*Device                   `json:"devices"`
	EntryPoint          string                      `json:"entry_point"`
	Inventory           []*Inventory                `json:"inventory"`
	Location            string                      `json:"location"`
	LocationEvent       *Event                      `json:"location_event"`
	Locations           map[string]*LocationSummary `json:"locations"`
	Moon                *Moon                       `json:"moon"`
	Moons               []*Moon                     `json:"moons"`
	MoonsScanned        int                         `json:"moons_scanned"`
	MoonsTotal          int                         `json:"moons_total"`
	MoonsTotalEstimated bool                        `json:"moons_total_estimated"`
	Object              *Object                     `json:"object"`
	Planet              *Planet                     `json:"planet"`
	Planets             []*Planet                   `json:"planets"`
	PlanetsScanned      int                         `json:"planets_scanned"`
	PlanetsTotal        int                         `json:"planets_total"`
	ResourceSites       []*Site                     `json:"resource_sites"`
	Star                *Star                       `json:"star"`
	SystemScanned       bool                        `json:"system_scanned"`
	Type                string                      `json:"location_type"`
}
