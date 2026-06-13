package models

import (
	"sort"
)

type Destination struct {
	Designation string  `json:"designation"`
	DistanceAu  float32 `json:"distanceAu"`
}

type Salvage struct {
	Depleted           bool     `json:"depleted"`
	Designation        string   `json:"designation"`
	Location           string   `json:"location"`
	Name               string   `json:"name"`
	ResourcesAvailable []string `json:"resources_available"`
	SalvageType        string   `json:"salvage_type"`
}

type ScanReplicant struct {
	LastActive string `json:"last_active"`
	Location   string `json:"location"`
	Name 	   string `json:"name"`
	// Don't alias other replicants
	Code       string `json:"replicant_code"`
}

type Object struct {
	ActivePlates      int     `json:"active_plates"`
	CurrentProgress   float32 `json:"current_progress"`
	Designation       string  `json:"designation"`
	ImpactEta         string  `json:"impact_eta"`
	ImpactTarget      string  `json:"impact_target"`
	ObjectType        string  `json:"object_type"`
	OrbitalDistanceAu float32 `json:"orbital_distance_au"`
	ProgressPct       float32 `json:"progress_pct"`
	RequiredStrength  float32 `json:"required_strength"`
	SizeClass         string  `json:"size_class"`
	Status            string  `json:"status"`
}

type LocationEvent struct {
	Designation string `json:"designation"`
	EventType   string `json:"event_type"`
	Location    string `json:"location"`
	Tier        int    `json:"tier"`
	Title       string `json:"title"`
}

type Trade struct {
	Criteria struct {
		Resources map[string]int `json:"resources"`
	}                            `json:"criteria"`
	CurrentStock int             `json:"current_stock"`
	Name string                  `json:"name"`
	Rewards struct {
		Devices map[string]int   `json:"devices"`
	}                            `json:"rewards"`
	TradeCode string             `json:"trade_code"`
}

type Shop struct {
	ControllerCode     string   `json:"controller_code"`
	Description        string   `json:"description"`
	Location           string   `json:"location"`
	LocationName       string   `json:"location_name"`
	OwnerName          string   `json:"owner_name"`
	OwnerReplicantCode string   `json:"owner_replicant_code"`
	ShopName           string   `json:"shop_name"`
	Trades             []*Trade `json:"trades"`
}

type Scan struct {
	ActiveLocationEvents []LocationEvent `json:"active_location_events"`
	AsteroidBelt         struct {
		Belts   []*Belt `json:"belts"`
		Present bool    `json:"present"`
	} `json:"asteroid_belt"`
	EntryPoint     string `json:"entry_point"`
	LifeDetected   bool   `json:"life_detected"`
	MiningBonusPct int    `json:"mining_bonus_pct"`
	OuterSystem    struct {
		Kuiper *Destination `json:"kuiper"`
		Oort   *Destination `json:"oort"`
	} `json:"outer_system"`
	Planets       []*Planet        `json:"planets"`
	Replicants    []*ScanReplicant `json:"replicants"`
	Shops 		  []*Shop          `json:"shops"`
	Star          *Star            `json:"star"`
	SystemObjects []*Object        `json:"system_objects"`
	SystemTags    []string         `json:"system_tags"`
}

func (s *Scan) ExtractLocations() []string {
	if s == nil {
		return nil
	}
	locs := make(map[string]bool)
	a := func(loc string) { locs[loc] = true }
	if s.EntryPoint != "" {
		a(s.EntryPoint)
	}
	a(s.Star.Designation)
	for _, b := range s.AsteroidBelt.Belts {
		a(b.Designation)
	}
	if s.OuterSystem.Oort.Designation != "" {
		a(s.OuterSystem.Oort.Designation)
	}
	if s.OuterSystem.Kuiper.Designation != "" {
		a(s.OuterSystem.Kuiper.Designation)
	}
	for _, p := range s.Planets {
		a(p.Designation)
	}
	for _, v := range s.Replicants {
		a(v.Location)
	}
	for _, o := range s.SystemObjects {
		a(o.Designation)
	}

	// Dedup and sort before returning.
	var res []string
	for l := range locs {
		res = append(res, l)
	}

	return sort.StringSlice(res)
}
