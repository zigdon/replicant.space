package models

import (
	"fmt"

	"encoding/json"
)

type DeviceInfo struct {
	AttachedToDeviceCode string `json:"attached_to_device_code"`
	AvailableCommands []string `json:"available_commands"`
	ControllerDeviceCode string `json:"controller_device_code"`
	DeviceCode string `json:"device_code"`
	DeviceType string `json:"device_type"`
	Features []string `json:"features"`
	Location string `json:"location"`
	Location_name string `json:"location_name"`
	OperationalCapacity float32 `json:"operational_capacity"`
	QueueSize int `json:"queue_size"`
	ReplicantCode string `json:"replicant_code"`
	Status string `json:"status"`
	StowedInDeviceCode string `json:"stowed_in_device_code"`
}

func ParseDeviceInfo(data []byte) (*DeviceInfo, error) {
	di := &DeviceInfo{}
	if err := json.Unmarshal(data, di); err != nil {
		return nil, fmt.Errorf("Error parsing device info: %v", err)
	}

	return di, nil
}
