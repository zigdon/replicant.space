package models


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
