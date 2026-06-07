package models

import (
	"fmt"

	"encoding/json"
)

func Parse[T any](data []byte) (*T, error) {
	s := new(T)

	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("Error parsing %T: %v", s, err)
	}

	return s, nil
}
