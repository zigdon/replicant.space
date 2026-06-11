package rest

import (
	"fmt"
	"time"

	"github.com/zigdon/rsp/models"
)

func durationFromSeconds(s float32) time.Duration {
	d, err := time.ParseDuration(fmt.Sprintf("%.2fs", s))
	if err != nil {
		log("Error parsing duration %v: %v", s, err)
	}
	return d
}

func fill[T []E, E models.Fillable](s []E) error {
	for _, e := range s {
		if err := e.Fill(); err != nil {
			return err
		}
	}
	return nil
}
