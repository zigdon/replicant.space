package rest

import (
	"fmt"
	"time"
)

func durationFromSeconds(s float32) time.Duration {
	d, err := time.ParseDuration(fmt.Sprintf("%.2fs", s))
	if err != nil {
		log("Error parsing duration %v: %v", s, err)
	}
	return d
}
