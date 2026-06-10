package rest

import (
	"encoding/json"
	"fmt"

	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
)

var db *cache.Cache

func ConnectDB(cdb *cache.Cache) {
	db = cdb
}

// Account
func Account() (*models.Account, error) {
	res, err := cacheGET("", 0, "accounts/me")
	if err != nil {
		return nil, err
	}
	acc, err := models.Parse[models.Account](res)
	if err != nil {
		return nil, err
	}
	acc.Replicants = make(map[string]*models.Replicant)
	for _, r := range acc.ReplicantList {
		acc.Replicants[r.Name] = r
	}
	return acc, nil
}

func PatchSettings(up *models.AccountUpdate) (*models.Account, error) {
	data, err := json.Marshal(up)
	if err != nil {
		return nil, err
	}
	res, err := Patch("accounts/me", data)
	acc, err := models.Parse[models.Account](res)
	if err != nil {
		return nil, err
	}
	acc.Replicants = make(map[string]*models.Replicant)
	for _, r := range acc.ReplicantList {
		acc.Replicants[r.Name] = r
	}
	return acc, nil
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

func MarkRead(ids []int) error {
	data, _ := json.Marshal(map[string][]int{
		"ids": ids,
	})
	_, err := Post("messages/read", data)
	return err
}

func Bobnet(relayID string, cursor, limit int, latest, npcs bool) (*models.Bobs, error) {
	res, err := Get("devices/%s/messages?cursor=%d&limit=%d&latest=%v&include_npcs=%v",
		relayID, cursor, limit, latest, npcs,
	)
	if err != nil {
		return nil, err
	}

	return models.Parse[models.Bobs](res)
}

func Events() (*models.Events, error) {
	res, err := Get("accounts/events")
	if err != nil {
		return nil, err
	}

	return models.Parse[models.Events](res)
}

func CompleteEvent(eid string) (*models.Event, error) {
	events, err := Events()
	if err != nil {
		return nil, err
	}
	var location string
	for _, e := range events.Events {
		if eid != "" && e.Designation != eid {
			continue
		}
		location = e.Location
		break
	}
	if location == "" {
		return nil, fmt.Errorf("can't find location for %q", eid)
	}
	res, err := Post("locations/%s/events/%s", nil, location, eid)
	if err != nil {
		return nil, err
	}

	ev, err := models.Parse[models.Event](res)
	if err == nil && ev.Error != "" {
		err = fmt.Errorf("Event error: %v", ev.Error)
	}
	return ev, err
}

// Replicants
func ReplicantID(id int) (string, error) {
	account, err := Account()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s-%d", account.Name, id)
	var names []string
	for _, r := range account.Replicants {
		if r.Name == name {
			return r.ReplicantCode.String(), nil
		}
		names = append(names, r.Name)
	}
	return "", fmt.Errorf("no replicant %q found in %q", name, names)
}

func ReplicantEvents(id string, cursor, limit int, latest bool, eventType, deviceType, deviceCode string) (*models.ReplicantEvents, error) {
	id = db.Dealias(id)
	res, err := Get("replicants/%s/events?cursor=%d&limit=%d&latest=%v&event_type=%s&device_type=%s&device_code=%s",
		id, cursor, limit, latest, eventType, deviceType, deviceCode,
	)
	if err != nil {
		return nil, err
	}

	return models.Parse[models.ReplicantEvents](res)
}

func ReplicantScan(id string) (*models.Scan, error) {
	id = db.Dealias(id)
	res, err := cachePOST("", 0, "replicants/%s/scan", nil, id)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Scan](res)
}

func ReplicantCensus(id string, cnt, page int) (*models.Census, error) {
	id = db.Dealias(id)
	res, err := cacheGET(fmt.Sprintf("%s-census-%d-%d", id, cnt, page), 0, "replicants/%s/stars?per_page=%d&page=%d", id, cnt, page)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Census](res)
}

func Replicant(id string) (*models.Replicant, error) {
	id = db.Dealias(id)
	res, err := cacheGET("", 0, "replicants/%s", id)
	if err != nil {
		return nil, err
	}
	r, err := models.Parse[models.Replicant](res)
	if err != nil {
		return nil, err
	}
	if r.CurrentLocation == "" {
		r.CurrentLocation = r.Location
	}
	if r.CurrentLocationName == "" {
		r.CurrentLocationName = r.LocationName
	}
	if r.Travel != nil {
		r.Travel.Eta = durationFromSeconds(r.Travel.EtaSeconds)
		r.Travel.TotalTime = durationFromSeconds(r.Travel.TotalTimeSeconds)
		for i, l := range r.Travel.Route {
			l.Time = durationFromSeconds(l.TimeSeconds)
			r.Travel.Route[i] = l
		}
	}
	return r, nil
}

func ReplicantDevices(id, loc string) ([]*models.Device, error) {
	id = db.Dealias(id)
	var q string
	if loc != "" {
		q = fmt.Sprintf("?location=%s", loc)
	}
	res, err := cacheGET("", 0, "replicants/%s/devices%s", id, q)
	if err != nil {
		return nil, err
	}
	devs, err := models.Parse[models.OwnedDevices](res)
	if err != nil {
		return nil, err
	}
	return devs.Devices, nil
}

func ReplicantTravel(id, dest string) (*models.Trip, error) {
	id = db.Dealias(id)
	data, _ := json.Marshal(map[string]string{
		"destination": dest,
	})
	trip, err := Post("replicants/%s/travel", data, id)
	if err != nil {
		return nil, err
	}
	m, err := models.Parse[models.Trip](trip)
	if err == nil && m.Error != "" {
		err = fmt.Errorf("Travel error: %v", m.Error)
		return m, err
	}
	m.TotalTime = durationFromSeconds(m.TotalTimeSeconds)
	for i, l := range m.Route {
		l.Time = durationFromSeconds(l.TimeSeconds)
		m.Route[i] = l
	}
	return m, err
}

// Devices
func DeviceCommand(id, command string, args map[string]any) (*models.CommandResp, error) {
	id = db.Dealias(id)
	if command == "" || id == "" {
		return nil, fmt.Errorf("id and command are required")
	}
	if args == nil {
		args = make(map[string]any)
	}
	// If there are any args that are aliases, replace them with the original values
	for k, v := range args {
		switch v.(type) {
		case string:
			args[k] = db.Dealias(v.(string))
		case []string:
			var res []string
			for _, i := range v.([]string) {
				res = append(res, db.Dealias(i))
			}
			args[k] = res
		}
	}
	args["command"] = command
	data, _ := json.Marshal(args)
	trip, err := Post("devices/%s", data, id)
	if err != nil {
		return nil, err
	}
	m, err := models.Parse[models.CommandResp](trip)
	if m != nil {
		m.Eta = durationFromSeconds(m.EtaSeconds)
		m.TotalTime = durationFromSeconds(m.TotalTimeSeconds)
	}
	return m, err
}

func DeviceInfo(id string) (*models.Device, error) {
	id = db.Dealias(id)
	res, err := cacheGET("", 0, "devices/%s", id)
	if err != nil {
		return nil, err
	}
	m, err := models.Parse[models.Device](res)
	if err != nil {
		return nil, err
	}
	if m.Printing != nil {
		m.Printing.EtaSeconds = durationFromSeconds(m.Printing.EtaRaw)
	}
	if m.Travel != nil {
		m.Travel.Eta = durationFromSeconds(m.Travel.EtaSeconds)
	}
	if m.Scan != nil {
		m.Scan.Eta = durationFromSeconds(m.Scan.EtaSeconds)
	}
	return m, nil
}

func DeviceNetwork(id string) (*models.Network, error) {
	id = db.Dealias(id)
	res, err := cacheGET("", 0, "devices/%s/network", id)
	if err != nil {
		return nil, err
	}

	n, err := models.Parse[models.Network](res)
	if err == nil && n.Error != "" {
		err = fmt.Errorf("Network error: %v", n.Error)
	}
	return n, err
}

func GetType(id string) (string, error) {
	dev, err := DeviceInfo(id)
	if err != nil {
		return "", err
	}
	return dev.Type, nil
}

func GetTagged(tag string) (*models.TaggedDevices, error) {
	res, err := cacheGET("", 0, "devices/tags/%s", tag)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.TaggedDevices](res)
}

// Inventory
func Location(id string) (*models.Location, error) {
	url := "locations"
	if id != "" {
		url += "/" + id
	}
	res, err := cacheGET("", 0, url)
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

func ReplicantPrint(id, command, device string) (*models.PrintResp, error) {
	id = db.Dealias(id)
	data := make(map[string]string)
	if command != "" {
		data["command"] = command
	}
	if device != "" {
		data["device_type"] = device
	}
	bytes, _ := json.Marshal(data)
	queue, err := Post("replicants/%s/print", bytes, id)
	if err != nil {
		return nil, err
	}
	m, err := models.Parse[models.PrintResp](queue)
	m.PrintTime = durationFromSeconds(m.PrintTimeSeconds)
	return m, err
}
