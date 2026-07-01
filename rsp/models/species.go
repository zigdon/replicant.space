package models

type Species struct {
	Species []struct {
		Description   string `json:"description"`
		Government    string `json:"government"`
		Greeting      string `json:"greeting"`
		HomeworldType string `json:"homeworld_type"`
		Name          string `json:"name"`
		SpeciesKey    string `json:"species_key"`
		TechAffinity  string `json:"tech_affinity"`
		Trait         string `json:"trait"`
	} `json:"Species"`
}
