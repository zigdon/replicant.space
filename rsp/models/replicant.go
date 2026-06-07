package models

type ReplicantEvent struct {
	CreatedAt  string         `json:"created_at"`
	DeviceCode *CodeAlias     `json:"device_code"`
	DeviceType string         `json:"device_type"`
	Type       string         `json:"event_type"`
	Message    string         `json:"message"`
	Payload    map[string]any `json:"payload"`
}

type ReplicantEvents struct {
	ReplicantEvents []ReplicantEvent `json:"events"`
}

type OwnedDevices struct {
	Devices []Device `json:"devices"`
}

type PrintQueue struct {
	DeviceType string `json:"device_type"`
	Notify     struct {
		Device  string `json:"device"`
		Email   bool   `json:"email"`
		Webhook bool   `json:"webhook"`
	} `json:"notify"`
}

type MissingResources struct {
	Have int `json:"have"`
	Need int `json:"need"`
}

type TravelLeg struct {
	Active       bool    `json:"active"`
	DistanceAU   float32 `json:"distance_au"`
	From         string  `json:"from"`
	From_name    string  `json:"from_name"`
	Leg          int     `json:"leg"`
	Time_seconds float32 `json:"time_seconds"`
	To           string  `json:"to"`
	ToName       string  `json:"to_name"`
	Type         string  `json:"type"`
}

type Travel struct {
	ArrivesAt        string      `json:"arrives_at"`
	DepartedAt       string      `json:"departed_at"`
	Destination      string      `json:"destination"`
	DestinationName  string      `json:"destination_name"`
	DestinationType  string      `json:"destination_type"`
	Eta_seconds      float32     `json:"eta_seconds"`
	Origin           string      `json:"origin"`
	OriginName       string      `json:"origin_name"`
	ProgressPercent  float32     `json:"progress_percent"`
	Route            []TravelLeg `json:"route"`
	Stage            string      `json:"stage"`
	TotalDistanceLy  float32     `json:"total_distance_ly"`
	TotalTimeSeconds float32     `json:"total_time_seconds"`
	Type             string      `json:"type"`
}

type Replicant struct {
	AttachedDevices     []string                    `json:"attached_devices"`
	Cargo               []string                    `json:"cargo"`
	CreatedAt           string                      `json:"created_at"`
	CurrentLocation     string                      `json:"current_location"`
	CurrentLocationName string                      `json:"current_location_name"`
	CurrentStar         string                      `json:"current_star"`
	CurrentStarName     string                      `json:"current_star_name"`
	Description         string                      `json:"description"`
	DeviceCount         int                         `json:"device_count"`
	ExperiencePoints    int                         `json:"experience_points"`
	HostedDeviceCode    *CodeAlias                  `json:"hosted_device_code"`
	IsNPC               bool                        `json:"is_npc"`
	Location            string                      `json:"location"`
	LocationName        string                      `json:"location_name"`
	Name                string                      `json:"name"`
	Plan                string                      `json:"plan"`
	Position            Position                    `json:"position"`
	PrintQueue          []PrintQueue                `json:"print_queue"`
	Project             string                      `json:"project"`
	Pronouns            string                      `json:"pronouns"`
	ReplicantCode       *CodeAlias                  `json:"replicant_code"`
	Status              string                      `json:"status"`
	StowedDevices       []Device                    `json:"stowed_devices"`
	Travel              Travel                      `json:"travel"`
	WaitingFor          map[string]MissingResources `json:"waiting_for"`
}

func (r *Replicant) GetDeviceIDs() []string {
	var res []string
	for _, d := range r.StowedDevices {
		res = append(res, d.Code.String())
	}
	return res
}
