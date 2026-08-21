package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

func ParseCube(cube cache.Position) *Position {
	return &Position{
		X: cube.X,
		Y: cube.Y,
		Z: cube.Z,
	}
}

func ParsePosition(coords string) (*Position, error) {
	p := new(Position)
	_, err := fmt.Sscanf(coords, "%f,%f,%f", &p.X, &p.Y, &p.Z)
	if err == nil {
		return p, err
	}

	_, err = fmt.Sscanf(coords, "%f:%f:%f", &p.X, &p.Y, &p.Z)
	if err == nil {
		return p, err
	}
	return nil, fmt.Errorf("Unable to parse position %q: %v", coords, err)
}

func (p *Position) AsCube() cache.Position {
	return cache.Position{X: p.X, Y: p.Y, Z: p.Z}
}

func (p *Position) Distance(to *Position) float32 {
	if p == nil || to == nil {
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

func (p *Position) UnmarshalJSON(data []byte) error {
	if p == nil {
		p = new(Position)
	}

	obj := make(map[string]float32)
	// First try to parse as an object
	if err := json.Unmarshal(data, &obj); err == nil {
		(*p).X = obj["x"]
		(*p).Y = obj["y"]
		(*p).Z = obj["z"]
		return nil
	}

	// Failing that, read as an array
	var coords []float32
	if err := json.Unmarshal(data, &coords); err != nil {
		return err
	}
	if len(coords) != 3 {
		return fmt.Errorf("Invalid position %v", coords)
	}
	(*p).X = coords[0]
	(*p).Y = coords[1]
	(*p).Z = coords[2]

	return nil
}

type StarCatalog struct {
	Total        int       `json:"total"`
	Generated_at *JSONTime `json:"generated_at"`
	Stars        []*Star   `json:"stars"`
}

func NewStar(id string) (*Star, error) {
	s := &Star{Designation: LocationID(id)}
	err := s.Get()
	if err != nil {
		return s, fmt.Errorf("Can't load star %q: %v", id, err)
	}
	return s, nil
}

type Star struct {
	AgeMy                 float32        `json:"age_my"`
	Color                 string         `json:"color"`
	Designation           LocationID     `json:"designation"`
	DistanceFromReplicant float32        `json:"distance_from_replicant"`
	DistanceFromSol       float32        `json:"distance_from_sol"`
	EntryPoint            LocationID     `json:"entry_point"`
	EstimatedPlanets      int            `json:"estimated_planets"`
	EstimatedTravelTime   *JSONTimeDelta `json:"estimated_travel_time"`
	Explored              bool           `json:"explored"`
	HabitableZone         struct {
		InnerAu float32 `json:"inner_au"`
		OuterAu float32 `json:"outer_au"`
	} `json:"habitable_zone"`
	HasHub          bool `json:"has_hub"`
	HasMyHub        bool
	HasLife         bool      `json:"has_life"`
	LuminositySolar float32   `json:"luminositysolar"`
	MassSolar       float32   `json:"mass_solar"`
	MiningBonusPct  int       `json:"mining_bonus_pct"`
	Name            string    `json:"name"`
	Position        *Position `json:"position"`
	SpectralType    string    `json:"spectral_type"`
	StellarClass    string    `json:"stellar_class"`
	TemperatureK    int       `json:"temperature_k"`
	Region          string    `json:"region"`
}

func (s *Star) Fill() error {
	if s.DistanceFromSol == 0 {
		s.DistanceFromSol = float32(math.Sqrt(
			float64(s.Position.X*s.Position.X) +
				float64(s.Position.Y*s.Position.Y) +
				float64(s.Position.Z*s.Position.Z)))
	}
	return nil
}

func (s *Star) Cache() error {
	if s == nil {
		return nil
	}
	cur := &Star{Designation: s.Designation}
	if err := cur.Get(); err == nil {
		if cur.EstimatedPlanets > 0 && cur.EstimatedPlanets != s.EstimatedPlanets {
			s.EstimatedPlanets = cur.EstimatedPlanets
		}
		s.Explored = cur.Explored || s.Explored
	}
	return db.Update(cache.StarsTable, map[string]any{
		"designation":   s.Designation,
		"entry_point":   s.EntryPoint,
		"est_planets":   s.EstimatedPlanets,
		"explored":      s.Explored,
		"has_hub":       s.HasHub,
		"has_life":      s.HasLife,
		"name":          s.Name,
		"position":      s.Position.AsCube(),
		"spectral_type": s.SpectralType,
		"region":        s.Region,
	})
}

func (s *Star) Get() error {
	if db == nil {
		return fmt.Errorf("Not connected to cache")
	}
	if s.Designation == "" {
		return fmt.Errorf("Can't load unknown star")
	}
	scan, err := db.Get(cache.StarsTable, s.Designation.Star())
	if err != nil {
		return fmt.Errorf("Error querying cache: %v", err)
	}
	if s.Position == nil {
		p := NewPosition(0, 0, 0)
		s.Position = p
	}
	var pos cache.Position
	if err := scan(&s.Designation, &s.Name, &s.EntryPoint, &s.EstimatedPlanets,
		&s.SpectralType, &s.Explored, &s.HasLife, &pos, &s.HasHub,
		&s.HasMyHub, &s.Region); err != nil {
		return err
	}
	s.Position = ParseCube(pos)
	return s.Fill()
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
	Designation   LocationID        `json:"designation"`
	InnerRadiusAu float32           `json:"inner_radius_au"`
	OuterRadiusAu float32           `json:"outer_radius_au"`
	Resources     map[string]string `json:"resources"`
	Star          LocationID
	Mining        bool
}

func (b *Belt) String() string {
	return fmt.Sprintf("%s (%s)", b.Designation, b.Density)
}

func (b *Belt) Cache() error {
	if b == nil {
		return nil
	}
	var errs []error
	errs = append(errs, db.Update(cache.BeltsTable, map[string]any{
		"designation": b.Designation,
		"star":        b.Star,
		"density":     b.Density,
		"resources":   cache.Encode(b.Resources),
	}))

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
	scan, err := db.Get(cache.BeltsTable, string(b.Designation))
	if err != nil {
		return fmt.Errorf("Error querying cache: %v", err)
	}
	var scannedRes cache.JSONB[map[string]string]
	errs = append(errs, scan(&b.Designation, &b.Star, &b.Density, &b.Mining, &scannedRes))
	b.Resources = scannedRes.Data

	return errors.Join(errs...)
}

type Site struct {
	Designation           LocationID     `json:"designation"`
	Index                 int            `json:"site_index"`
	Name                  string         `json:"name"`
	ResourcesRemainingPct map[string]int `json:"resources_remaining_pct"`
	Type                  string         `json:"site_type"`
}

type Planet struct {
	Atmosphere          bool         `json:"atmosphere"`
	AxialTiltDeg        float32      `json:"axial_tilt_deg"`
	DensityGcc          float32      `json:"density_gcc"`
	Designation         LocationID   `json:"designation"`
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
	Star                LocationID
}

func (p *Planet) Cache() error {
	if p == nil {
		return nil
	}
	if p.Star == "" {
		p.Star = LocationID(p.Designation.Star())
	}
	data := map[string]any{
		"designation": p.Designation,
		"star":        p.Star,
		"name":        p.Name,
		"moons":       p.MoonCount,
		"rings":       p.Rings,
		"scanned":     p.Scanned,
		"type":        p.Type,
	}
	if p.LifeStage != "" {
		data["life_stage"] = p.LifeStage
	}
	return db.Update(cache.PlanetsTable, data)
}

func (p *Planet) Get() error {
	if db == nil {
		return fmt.Errorf("Not connected to cache")
	}
	if p.Designation == "" {
		return fmt.Errorf("Can't load unknown planet")
	}
	scan, err := db.Get(cache.PlanetsTable, string(p.Designation))
	if err != nil {
		return fmt.Errorf("Error querying cache: %v", err)
	}
	return scan(&p.Designation, &p.Star, &p.Name, &p.LifeStage, &p.MoonCount,
		&p.Rings, &p.Scanned)
}

type Moon struct {
	Designation  LocationID `json:"designation"`
	Name         string     `json:"name"`
	ParentPlanet LocationID `json:"parent_planet"`
	Star         LocationID
	Scanned      bool   `json:"scanned"`
	Type         string `json:"location_type"`
}

func (m *Moon) Cache() error {
	if m == nil {
		return nil
	}
	if m.Star == "" {
		m.Star = LocationID(m.Designation.Star())
	}
	if m.ParentPlanet == "" {
		m.ParentPlanet = LocationID(strings.Join(strings.Split(string(m.Designation), "-")[:2], "-"))
	}
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
	scan, err := db.Get(cache.MoonsTable, string(m.Designation))
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
	AsteroidBelt *struct {
		Belts   []*Belt `json:"belts"`
		Present bool    `json:"present"`
	} `json:"asteroid_belt"`
	Belt                *Belt                           `json:"belt"`
	Devices             []*Device                       `json:"devices"`
	EntryPoint          LocationID                      `json:"entry_point"`
	Inventory           []*Inventory                    `json:"inventory"`
	Location            LocationID                      `json:"location"`
	LocationEvent       *Event                          `json:"location_event"`
	Locations           map[LocationID]*LocationSummary `json:"locations"`
	Moon                *Moon                           `json:"moon"`
	Moons               []*Moon                         `json:"moons"`
	MoonsScanned        int                             `json:"moons_scanned"`
	MoonsTotal          int                             `json:"moons_total"`
	MoonsTotalEstimated bool                            `json:"moons_total_estimated"`
	Object              *Object                         `json:"object"`
	Planet              *Planet                         `json:"planet"`
	Planets             []*Planet                       `json:"planets"`
	PlanetsScanned      int                             `json:"planets_scanned"`
	PlanetsTotal        int                             `json:"planets_total"`
	ResourceSites       []*Site                         `json:"resource_sites"`
	Star                *Star                           `json:"star"`
	SystemScanned       bool                            `json:"system_scanned"`
	Type                string                          `json:"location_type"`
}

func (l *Location) Fill() error {
	if l.EntryPoint != "" {
		l.Star.EntryPoint = l.EntryPoint
	}

	return nil
}

func (l *Location) Cache() error {
	var errs []error
	var objs []Cachable
	for _, c := range []Cachable{l.Star, l.Planet, l.Belt, l.Moon} {
		if c == nil {
			continue
		}
		objs = append(objs, c)
	}
	if l.AsteroidBelt != nil {
		for _, b := range l.AsteroidBelt.Belts {
			if b == nil {
				continue
			}
			b.Star = l.Star.Designation
			objs = append(objs, b)
		}
	}
	for _, m := range l.Moons {
		if m == nil {
			continue
		}
		objs = append(objs, m)
	}
	for _, p := range l.Planets {
		if p == nil {
			continue
		}
		objs = append(objs, p)
	}
	for _, o := range objs {
		errs = append(errs, o.Cache())
	}
	for k, v := range l.Locations {
		if v.Resources == 0 {
			_, err := db.Exec(`DELETE FROM inventory WHERE designation = $1`, k)
			errs = append(errs, err)
		}
	}
	if len(l.Inventory) > 0 {
		res := make(map[string]any)
		for _, r := range []string{"carbon", "conductive", "rares", "silicates", "structural", "volatiles"} {
			res[r] = 0
		}
		for _, i := range l.Inventory {
			res[i.ResourceType] = i.Quantity
		}
		res["designation"] = string(l.Location)
		res["star"] = l.Location.Star()
		errs = append(errs, db.Update(cache.InventoryTable, res))
	}
	return errors.Join(errs...)
}

func (l *Location) Get() error {
	return fmt.Errorf("Not implemented")
}
