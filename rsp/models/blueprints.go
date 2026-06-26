package models

type Blueprint struct {
	AttachCapacity   int            `json:"attach_capacity"`
	CargoCapacity    int            `json:"cargo_capacity"`
	DeviceType       string         `json:"device_type"`
	Description      string         `json:"description"`
	Directives       []string       `json:"directives"`
	Features         []string       `json:"features"`
	PrintTime        *JSONTimeDelta `json:"print_time"`
	Resources        map[string]int `json:"resources"`
	ShortDescription string         `json:"short_description"`
	StowCapacity     int            `json:"stow_capacity"`
	Strength         float32        `json:"strength"`
}

type Blueprints struct {
	Blueprints []*Blueprint `json:"blueprints"`
}

type PrintResp struct {
	Status            string         `json:"status"`
	DeviceType        string         `json:"device_type"`
	StartedAt         string         `json:"started_at"`
	CompletesAt       string         `json:"completes_at"`
	PrintTime         *JSONTimeDelta `json:"print_time_seconds"`
	ResourcesRefunded bool           `json:"resources_refunded"`
}

type Queued struct {
	Queue       []string `json:"queue"`
	QueueLength int      `json:"queue_length"`
	Status      string   `json:"status"`
}
