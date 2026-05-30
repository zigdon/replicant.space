package rest

import (
	"encoding/json"
	"fmt"

	"github.com/zigdon/rsp/models"
)

func Me() (*models.Me, error) {
	res, err := Get("accounts/me")
	if err != nil {
		return nil, err
	}
	return models.ParseMe(res)
}

func ReplicantScan(id string) (*models.Scan, error) {
	res, err := Post("replicants/%s/scan", nil, id)
	if err != nil {
		return nil, err
	}
	return models.ParseScan(res)
}

func Replicant(id string) (*models.Replicant, error) {
	res, err := Get("replicants/%s", id)
	if err != nil {
		return nil, err
	}
	return models.ParseReplicant(res)
}

func Travel(id, dest string) (*models.Trip, error) {
	if dest == "" || id == "" {
		return nil, fmt.Errorf("id and destination are required for travel")
	}
	data, _ := json.Marshal(map[string]string{
		"destination": dest,
	})
	trip, err := Post("replicants/%s/travel", data, id)
	if err != nil {
		return nil, err
	}
	return models.ParseTrip(trip)
}
