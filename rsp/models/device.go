package models

import (
	"fmt"

	"encoding/json"
)

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

