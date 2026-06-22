package models

type TripLeg struct {
	Active      bool          `json:"active"`
	DistanceAu  float32       `json:"distance_au"`
	DistanceLy  float32       `json:"distance_ly"`
	From        string        `json:"from"`
	FromName    string        `json:"from_name"`
	Leg         int           `json:"leg"`
	Time        JSONTimeDelta `json:"time_seconds"`
	To          string        `json:"to"`
	ToName      string        `json:"to_name"`
	Type        string        `json:"type"`
}

type Trip struct {
	Arrives          JSONTime `json:"arrives_at"`
	Departed         JSONTime `json:"departed_at"`
	Destination      string  `json:"destination"`
	DestinationName  string  `json:"destination_name"`
	DistanceLy       float32 `json:"distance_ly"`
	Error            string  `json:"error"`
	Eta              JSONTimeDelta `json:"eta_seconds"`
	Origin           string    `json:"origin"`
	OriginName       string    `json:"origin_name"`
	ProgressPercent  float32   `json:"progress_percent"`
	Route            []TripLeg `json:"route"`
	Status           string    `json:"status"`
	TotalTime        JSONTimeDelta `json:"total_time_seconds"`
	Type             string  `json:"type"`
}

type JourneyLeg struct {
	From         string
	FromPosition Position
	To           string
	ToPosition   Position
	DistFromSrc  float32
	DistToDest   float32
	Processed    bool
}

type Journey struct {
	Source         string
	SourcePosition Position
	Dest           string
	DestPosition   Position
	Legs           []JourneyLeg
}
