package rest

import (
	"github.com/zigdon/rsp/cfg"
    "github.com/zigdon/rsp/models"
)

func Me() (*models.Me, error) {
	res, err := Get("accounts/me")
	if err != nil {
		return nil, err
	}
	return models.ParseMe(res)
}

func ReplicantScan(id int) (*models.Scan, error) {
	rID := cfg.GetID(id)
	res, err := Post("replicants/%s/scan", nil, rID)
	if err != nil {
		return nil, err
	}
	return models.ParseScan(res)
}
