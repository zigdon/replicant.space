package models

import (
	"fmt"
	"time"

	"encoding/json"

	"github.com/zigdon/rsp/errors"
)

type DevicePrint struct {
	CompletesAt string `json:"completes_at"`
	DeviceType string `json:"device_type"`
	EtaRaw float32 `json:"eta_seconds"`
	EtaSeconds time.Duration
	ProgressPercent float32 `json:"progress_percent"`
	StartedAt string `json:"started_at"`
}

type StowedDevice struct {
	Code string `json:"device_code"`
	Type string `json:"device_type"`
}

type Device struct {
	AttachedToDeviceCode string `json:"attached_to_device_code"`
	AvailableCommands []string `json:"available_commands"`
	ControllerDeviceCode string `json:"controller_device_code"`
	Code string `json:"device_code"`
	Type string `json:"device_type"`
	Features []string `json:"features"`
	InControlRange bool `json:"in_control_range"`
	Location string `json:"location"`
	LocationName string `json:"location_name"`
	OperationalCapacity float32 `json:"operational_capacity"`
	QueueSize int `json:"queue_size"`
	ReplicantCode string `json:"replicant_code"`
	Status string `json:"status"`
	StowCapacity int `json:"stow_capacity"`
	StowedDevices []StowedDevice `json:"stowed_devices"`
	StowedInDeviceCode string `json:"stowed_in_device_code"`
	Printing DevicePrint `json:"printing"`
}

type CommandResp struct {
	Belt string `json:"belt"`
	CompletesAt string `json:"completes_at"`
	DeviceCode string `json:"device_code"`
	EtaRaw float32 `json:"eta_seconds"`
	EtaSeconds time.Duration
	Location string `json:"location"`
	Star string `json:"star"`
	StartedAt string `json:"started_at"`
	Status string `json:"status"`

	JsonErr string `json:"error"`
	AvailableSites []AvailableSite `json:"available_sites"`
}

type AvailableSite struct {
	Designation string `json:"designation"`
	Name string `json:"name"`
	SalvageType string `json:"salvage_type"`
}

func ParseDevice(data []byte) (*Device, error) {
	di := &Device{}
	if err := json.Unmarshal(data, di); err != nil {
		return nil, fmt.Errorf("Error parsing device info: %v", err)
	}
	di.Printing.EtaSeconds, _ = time.ParseDuration(fmt.Sprintf("%.2fs", di.Printing.EtaRaw))

	return di, nil
}

func ParseCommandResp(data []byte) (*CommandResp, error) {
	dc := &CommandResp{}
	if err := json.Unmarshal(data, dc); err != nil {
		return nil, fmt.Errorf("Error parsing command response: %v", err)
	}
	dc.EtaSeconds, _ = time.ParseDuration(fmt.Sprintf("%.2fs", dc.EtaRaw))

	if dc.JsonErr != "" {
		return dc, &errors.PostError{Err: fmt.Errorf("%s", dc.JsonErr)}
	}

	return dc, nil
}
