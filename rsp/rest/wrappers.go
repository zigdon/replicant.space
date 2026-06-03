package rest

import (
	"encoding/json"
	"fmt"

	"github.com/zigdon/rsp/models"
)

// / Account
func Account() (*models.Account, error) {
	res, err := cacheGET("", 0, "accounts/me")
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Account](res)
}

func Messages(cursor, limit int, latest, unreadOnly bool) (*models.Messages, error) {
	res, err := Get("messages?cursor=%d&limit=%d&latest=%v&unread_only=%v",
		cursor, limit, latest, unreadOnly,
	)
	if err != nil {
		return nil, err
	}

	return models.Parse[models.Messages](res)
}

// / Replicants
func ReplicantID(id int) (string, error) {
	account, err := Account()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s-%d", account.Name, id)
	var names []string
	for _, r := range account.Replicants {
		if r.Name == name {
			return r.ReplicantCode, nil
		}
		names = append(names, r.Name)
	}
	return "", fmt.Errorf("no replicant %q found in %q", name, names)
}

func ReplicantScan(id string) (*models.Scan, error) {
	res, err := cachePOST("", 0, "replicants/%s/scan", nil, id)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Scan](res)
}

func ReplicantCensus(id string, page int) (*models.Census, error) {
	res, err := cacheGET(fmt.Sprintf("%s-census", id), 0, "replicants/%s/stars?per_page=50&page=%d", id, page)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Census](res)
}

func Replicant(id string) (*models.Replicant, error) {
	res, err := cacheGET("", 0, "replicants/%s", id)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Replicant](res)
}

func ReplicantDevices(id string) ([]models.Device, error) {
	res, err := cacheGET("", 0, "replicants/%s/devices", id)
	if err != nil {
		return nil, err
	}
	devs, err := models.Parse[models.OwnedDevices](res)
	if err != nil {
		return nil, err
	}
	return devs.Devices, nil
}

func Travel(id, dest string) (*models.Trip, error) {
	data, _ := json.Marshal(map[string]string{
		"destination": dest,
	})
	trip, err := Post("replicants/%s/travel", data, id)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Trip](trip)
}

// / Devices
func DeviceCommand(id, command string, args map[string]any) (*models.CommandResp, error) {
	if command == "" || id == "" {
		return nil, fmt.Errorf("id and command are required")
	}
	if args == nil {
		args = make(map[string]any)
	}
	args["command"] = command
	data, _ := json.Marshal(args)
	trip, err := Post("devices/%s", data, id)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.CommandResp](trip)
}

func DeviceInfo(id string) (*models.Device, error) {
	res, err := cacheGET("", 0, "devices/%s", id)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Device](res)
}

/// Inventory
func Location(id string) (*models.Location, error) {
	res, err := cacheGET("", 0, "locations/%s", id)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Location](res)
}

func Blueprints() (*models.Blueprints, error) {
	res, err := cacheGET("", 0, "blueprints")
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Blueprints](res)
}

func Print(id, command, device string) (*models.PrintResp, error) {
	data := make(map[string]string)
	if command != "" { data["command"] = command }
	if device != "" { data["device_type"] = device }
	bytes, _ := json.Marshal(data)
	queue, err := Post("replicants/%s/print", bytes, id)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.PrintResp](queue)
}
