package models

import (
	"fmt"

	"encoding/json"
)

type Device struct {
	Code string `yaml:"device_code"`
	Type string `yaml:"device_type"`
	Location string `yaml:"location"`
	Status string `yaml:"status"`
	OperationalCapacity float32 `yaml:"operational_capacity"`
}

type CommandResp struct {
	DeviceCode string `yaml:"device_code"`
	Location string `yaml:"location"`
	Star string `yaml:"star"`
	Status string `yaml:"status"`
}

func ParseCommandResp(data []byte) (*CommandResp, error) {
	dc := &CommandResp{}
	if err := json.Unmarshal(data, dc); err != nil {
		return nil, fmt.Errorf("Error parsing command response: %v", err)
	}

	return dc, nil
}

