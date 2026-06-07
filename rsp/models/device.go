package models

import (
	"time"
)

type DevicePrint struct {
	CompletesAt     string  `json:"completes_at"`
	DeviceType      string  `json:"device_type"`
	EtaRaw          float32 `json:"eta_seconds"`
	EtaSeconds      time.Duration
	ProgressPercent float32 `json:"progress_percent"`
	StartedAt       string  `json:"started_at"`
}

type ControlledDevice struct {
	Code     string `json:"device_code"`
	Type     string `json:"device_type"`
	Location string `json:"location"`
	Status   string `json:"status"`
}

type StowedDevice struct {
	Code string `json:"device_code"`
	Type string `json:"device_type"`
}

type DeviceDirective struct {
	Config map[string]string `json:"config"`
	Name   string            `json:"name"`
}

type Device struct {
	AmiDirective         *DeviceDirective    `json:"ami_directive"`
	AmiDirectiveStatus   string              `json:"ami_directive_status"`
	AttachCapacity       int                 `json:"attach_capacity"`
	AttachedDevices      []*Device           `json:"attached_devices"`
	AttachedToDeviceCode string              `json:"attached_to_device_code"`
	AvailableCommands    []string            `json:"available_commands"`
	AvailableDirectives  []string            `json:"available_directives"`
	Cargo                []*Inventory        `json:"cargo"`
	Code                 string              `json:"device_code"`
	ControlledDevices    []*ControlledDevice `json:"controlled_devices"`
	ControllerDeviceCode string              `json:"controller_device_code"`
	Features             []string            `json:"features"`
	InControlRange       bool                `json:"in_control_range"`
	Location             string              `json:"location"`
	LocationName         string              `json:"location_name"`
	OperationalCapacity  float32             `json:"operational_capacity"`
	Printing             *DevicePrint        `json:"printing"`
	QueueSize            int                 `json:"queue_size"`
	ReplicantCode        string              `json:"replicant_code"`
	Status               string              `json:"status"`
	StowCapacity         int                 `json:"stow_capacity"`
	StowedDevices        []*StowedDevice     `json:"stowed_devices"`
	StowedInDeviceCode   string              `json:"stowed_in_device_code"`
	TaxiMode             string              `json:"taxi_mode"`
	Type                 string              `json:"device_type"`
}

type ControllerStatus struct {
	DirectivePaused       bool   `json:"directive_paused"`
	DirectiveStatusAfter  string `json:"directive_status_after"`
	DirectiveStatusBefore string `json:"directive_status_before"`
	RecallResult          string `json:"recall_result"`
}

type CommandResp struct {
	ArrivesAt            string              `json:"arrives_at"`
	AssignedDevices      map[string][]string `json:"assigned_devices"`
	AttachedDevices      []*Device           `json:"attached_devices"`
	AvailableSites       []*AvailableSite    `json:"available_sites"`
	Belt                 string              `json:"belt"`
	CompletesAt          string              `json:"completes_at"`
	Controller           *ControllerStatus   `json:"controller"`
	ControllerCode       string              `json:"controller_code"`
	DepartedAt           string              `json:"departed_at"`
	Destination          string              `json:"destination"`
	DestinationName      string              `json:"destination_name"`
	DestinationType      string              `json:"destination_type"`
	DeviceCode           string              `json:"device_code"`
	EtaSeconds           float32             `json:"eta_seconds"`
	Eta                  time.Duration
	FinalDestination     string          `json:" strin"`
	FinalDestinationName string          `json:"final_destination_name"`
	JsonErr              string          `json:"error"`
	Location             string          `json:"location"`
	Moon                 *Moon           `json:"moon"`
	Origin               string          `json:"origin"`
	OriginName           string          `json:"origin_name"`
	ProgressPercent      float32         `json:"progress_percent"`
	Released             []*StowedDevice `json:"released"`
	Route                []*TripLegs     `json:"route"`
	Scanned              bool            `json:"scanned"`
	Star                 string          `json:"star"`
	StartedAt            string          `json:"started_at"`
	Status               string          `json:"status"`
	TotalDistanceLy      float32         `json:" float3"`
	TotalTime            time.Duration
	TotalTimeSeconds     float32 `json:"total_time_seconds"`
	TravelType           string  `json:"travel_type"`
}

type AvailableSite struct {
	Designation string `json:"designation"`
	Name        string `json:"name"`
	SalvageType string `json:"salvage_type"`
}
