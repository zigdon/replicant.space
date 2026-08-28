package common

import (
	"errors"
	"fmt"

	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
)

func GetUpkeep() (map[string]map[string]int, error) {
	res := make(map[string]map[string]int)
	var errs []error
	// Find all the system hubs, collect their upkeep requirements
	rows, err := db.Query(`
		SELECT location, data->'upkeep_requirements'
		FROM json_devices
		WHERE type = 'system_hub'
		  AND location != ''
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var l string
		var up cache.JSONB[[]*models.UpkeepRequirement]
		if err := rows.Scan(&l, &up); err != nil {
			errs = append(errs, fmt.Errorf("Error scanning upkeep: %v", err))
			continue
		}
		if _, ok := res[l]; !ok {
			res[l] = make(map[string]int)
		}
		for _, r := range up.Data {
			res[l][r.ResourceType] += r.QuantityPer20pct
		}
	}

	return res, errors.Join(errs...)
}
