package models

import (
	"fmt"

	"encoding/json"
)

type Device struct {
	Code string `json:"device_code"`
	ControllerDeviceCode string `json:"controller_device_code"`
	Features []string `json:"features"`
	InControlRange bool `json:"in_control_range"`
	Location string `json:"location"`
	LocationName string `json:"location_name"`
	OperationalCapacity float32 `json:"operational_capacity"`
	Status string `json:"status"`
	StowedInDeviceCode string `json:"stowed_in_device_code"`
	Type string `json:"device_type"`
}

type CommandResp struct {
	DeviceCode string `json:"device_code"`
	Location string `json:"location"`
	Star string `json:"star"`
	Status string `json:"status"`
}

func ParseCommandResp(data []byte) (*CommandResp, error) {
	dc := &CommandResp{}
	if err := json.Unmarshal(data, dc); err != nil {
		return nil, fmt.Errorf("Error parsing command response: %v", err)
	}

	return dc, nil
}

