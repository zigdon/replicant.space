package rest

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	acc.UpdateFn = Account
	return acc, nil
}

func PatchSettings(up *models.AccountUpdate) (*models.Account, error) {
	data, err := json.Marshal(up)
	if err != nil {
		return nil, err
	}
	res, err := Patch("accounts/me", data)
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
		location = string(e.Location)
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

func StarCatalog() (*models.StarCatalog, error) {
	res, err := cacheGET("", 5*time.Minute, "stars")
	if err != nil {
		return nil, err
	}
	return models.Parse[models.StarCatalog](res)
}

// Replicants
func ReplicantID(id int) (*models.CodeAlias, error) {
	account, err := Account()
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s-%d", account.Name, id)
	var names []string
	for _, r := range account.Replicants {
		if r.Name == name {
			return r.Code, nil
		}
		names = append(names, r.Name)
	}
	return nil, fmt.Errorf("no replicant %q found in %q", name, names)
}

func ReplicantEvents(id *models.CodeAlias, cursor, limit int, latest bool, eventType, deviceType, deviceCode string) (*models.ReplicantEvents, error) {
	res, err := Get("replicants/%s/events?cursor=%d&limit=%d&latest=%v&event_type=%s&device_type=%s&device_code=%s",
		id, cursor, limit, latest, eventType, deviceType, deviceCode,
	)
	if err != nil {
		return nil, err
	}

	return models.Parse[models.ReplicantEvents](res)
}

func ReplicantScan(id *models.CodeAlias) (*models.Scan, error) {
	res, err := cachePOST("", 0, "replicants/%s/scan", nil, id)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Scan](res)
}

func ReplicantCensus(id *models.CodeAlias, cnt, page int) (*models.Census, error) {
	res, err := cacheGET(fmt.Sprintf("%s-census-%d-%d", id, cnt, page), 0, "replicants/%s/stars?per_page=%d&page=%d", id, cnt, page)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Census](res)
}

func Replicant(id *models.CodeAlias) (*models.Replicant, error) {
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
	r.UpdateFn = Replicant
	return r, nil
}

func ReplicantDevices(c *models.CodeAlias, loc string) ([]*models.Device, error) {
	var q string
	if loc != "" {
		q = fmt.Sprintf("?location=%s", loc)
	}
	res, err := cacheGET("", 0, "replicants/%s/devices%s", c.String(), q)
	if err != nil {
		return nil, err
	}
	devs, err := models.Parse[models.OwnedDevices](res)
	if err != nil {
		return nil, err
	}
	return devs.Devices, nil
}

func ReplicantTravel(id *models.CodeAlias, dest string, via []string, dryRun bool) (*models.Trip, error) {
	cfg := map[string]any{
		"dry_run":     dryRun,
		"destination": dest,
	}
	if len(via) > 0 {
		cfg["via"] = via
	}
	data, _ := json.Marshal(cfg)
	trip, err := Post("replicants/%s/travel", data, id)
	if err != nil {
		return nil, err
	}
	m, err := models.Parse[models.Trip](trip)
	if err == nil && m.Error != "" {
		err = fmt.Errorf("Travel error: %v", m.Error)
	}
	m.Device = id
	m.Notification()
	return m, err
}

func ReplicantTeleport(id, target *models.CodeAlias) (*models.Teleport, error) {
	data, _ := json.Marshal(map[string]string{
		"target": target.String(),
	})
	trip, err := Post("replicants/%s/teleport", data, id)
	if err != nil {
		return nil, err
	}
	m, err := models.Parse[models.Teleport](trip)
	if err == nil && m.Error != "" {
		err = fmt.Errorf("Teleport error: %v", m.Error)
	}
	return m, err
}

// Devices
func CachedDevices(filters map[string]string, useCache bool) ([]*models.Device, error) {
	if !useCache {
		return RefreshDevices(filters)
	}

	if len(filters) == 0 {
		if cached, err := db.DB.Query(`
			SELECT code, data
			FROM json_devices`); err == nil {
			var devs []*models.Device
			valid := true
			for cached.Next() {
				var code string
				var data []byte
				if err := cached.Scan(&code, &data); err != nil {
					log("Error scanning %q: %v", code, err)
					valid = false
					break
				}
				d, err := models.Parse[models.Device](data)
				if err != nil {
					log("Error parsing %q: %v", code, err)
					valid = false
					break
				}
				devs = append(devs, d)
				if time.Since(d.Fetched()) > time.Minute {
					log("Cache for %s too old: %s (%s)", d.Code.Alias(), d.Fetched(), time.Since(d.Fetched()))
					valid = false
					break
				}
			}
			if err := cached.Err(); err != nil {
				log("Error in cache: %v", err)
				valid = false
			}
			if valid {
				return devs, nil
			}
		}
	}

	// Implement filtering from the cached devices
	// device_type
	// location

	return RefreshDevices(filters)
}

func Devices(filters map[string]string) ([]*models.Device, error) {
	return CachedDevices(filters, true)
}

func RefreshDevices(filters map[string]string) ([]*models.Device, error) {
	url := "devices"
	var params []string
	for k, v := range filters {
		params = append(params, fmt.Sprintf("%s=%s", k, v))
	}
	params = append([]string{"limit=50", "cursor=%d"}, params...)
	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}
	cur := 0
	var devs []*models.Device
	for {
		resp, err := Get(url, cur)
		if err != nil {
			return nil, err
		}
		ds, err := models.Parse[models.TaggedDevices](resp)
		if err != nil {
			return nil, err
		}

		devs = append(devs, ds.Devices...)
		if ds.NextCursor == 0 {
			break
		}
		cur = ds.NextCursor
	}

	return devs, nil
}

func DeviceCommand[T any](id *models.CodeAlias, command string, args map[string]any) (*T, error) {
	if command == "" || id == nil {
		return nil, fmt.Errorf("id and command are required")
	}
	if args == nil {
		args = make(map[string]any)
	}
	// If there are any args that are aliases, replace them with the original values
	for k, v := range args {
		switch v := v.(type) {
		case string:
			args[k] = db.Dealias(v)
		case []string:
			var res []string
			for _, i := range v {
				res = append(res, db.Dealias(i))
			}
			args[k] = res
		}
	}
	args["command"] = command
	data, _ := json.Marshal(args)
	resp, err := Post("devices/%s", data, id)
	if err != nil {
		return nil, err
	}
	return models.Parse[T](resp)
}

func DeviceLogs(id *models.CodeAlias, latest bool, page, limit int) (*models.DeviceLogs, error) {
	var res []byte
	var err error
	skip := page * limit
	if latest {
		res, err = cacheGET("", 0, "devices/%s/logs?latest=%v", id, latest)
		if err != nil {
			return nil, err
		}
		return models.Parse[models.DeviceLogs](res)
	}
	ret := new(models.DeviceLogs)
	var cursor = 0
	for {
		res, err = cacheGET("", 0, "devices/%s/logs?limit=%d&cursor=%d", id, limit, cursor)
		if err != nil {
			return nil, err
		}
		logs, err := models.Parse[models.DeviceLogs](res)
		if err != nil {
			return nil, err
		}
		ret.Events = append(ret.Events, logs.Events...)
		if len(ret.Events) > skip+limit {
			break
		}
		if logs.NextCursor == 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
		cursor = logs.NextCursor
	}
	if skip > 0 {
		ret.Events = ret.Events[skip:len(ret.Events)]
	}
	if len(ret.Events) > limit {
		ret.Events = ret.Events[0:limit]
	}
	return ret, nil
}

func RefreshDeviceInfo(id *models.CodeAlias) (*models.Device, error) {
	res, err := Get("devices/%s", id)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Device](res)
}

func CachedDeviceInfo(id *models.CodeAlias, useCache bool) (*models.Device, error) {
	if !useCache {
		log("Skipping cache for %q", id.Alias())
		return RefreshDeviceInfo(id)
	}
	d := &models.Device{Code: id}
	if err := d.Get(); err != nil {
		log("Error loading cached %s: %v", id.Alias(), err)
		return RefreshDeviceInfo(id)
	}
	if time.Since(d.Fetched()) <= time.Minute {
		log("Using cache for %q", id.Alias())
		return d, nil
	}

	log("Using cache for %q", id.Alias())
	return RefreshDeviceInfo(id)
}

func DeviceInfo(id *models.CodeAlias) (*models.Device, error) {
	return CachedDeviceInfo(id, true)
}

func DeviceNetwork(id *models.CodeAlias) (*models.Network, error) {
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

func Prospect(id *models.CodeAlias, dir *models.Position) (*models.Prospect, error) {
	cfg := map[string]any{
		"command": "prospect",
	}
	if dir != nil {
		cfg["direction"] = []float32{dir.X, dir.Y, dir.Z}
	}
	cfgData, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	res, err := Post("devices/%s", cfgData, id.String())
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Prospect](res)
}

func GetType(code string) (string, error) {
	if code == "" {
		return "", fmt.Errorf("can't get type of blank")
	}
	dev, err := DeviceInfo(models.NewCodeAlias(code))
	if err != nil {
		return "", err
	}
	return dev.Type, nil
}

type TagOp string

const (
	SetTags TagOp = "tags"
	AddTag  TagOp = "add_tags"
	DelTag  TagOp = "remove_tags"
)

func UpdateTags(id *models.CodeAlias, operation TagOp, tags []string) (*models.Device, error) {
	data, err := json.Marshal(map[string]any{
		"configuration": map[string][]string{
			string(operation): tags,
		},
	})
	if err != nil {
		return nil, err
	}
	res, err := Patch("devices/%s", data, id)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Device](res)
}

func GetTagged(tag string) (*models.TaggedDevices, error) {
	var cursor int
	all := new(models.TaggedDevices)
	for {
		res, err := cacheGET("", 5*time.Minute, "devices/tags/%s?limit=5&cursor=%d", tag, cursor)
		if err != nil {
			return nil, err
		}
		t, err := models.Parse[models.TaggedDevices](res)
		if err != nil {
			return all, err
		}
		all.Devices = append(all.Devices, t.Devices...)

		if t.NextCursor == 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
		cursor = t.NextCursor
	}

	return all, nil
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

func Blueprints(refresh bool) (*models.Blueprints, error) {
	if !refresh && db != nil {
		bps := &models.Blueprints{}
		if err := bps.Get(); err != nil {
			return nil, err
		}
		return bps, nil
	}
	res, err := cacheGET("", 30*time.Minute, "blueprints")
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Blueprints](res)
}

func ReplicantPrint(id *models.CodeAlias, command, device string) (*models.PrintResp, error) {
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
	return models.Parse[models.PrintResp](queue)
}

// Trades
func Traders(id *models.CodeAlias) (*models.Shops, error) {
	res, err := cacheGET(fmt.Sprintf("traders-%s", id), 0, "replicants/%s/traders", id)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Shops](res)
}

func Trades(sid string) (*models.Shop, error) {
	res, err := cacheGET(fmt.Sprintf("trades-%s", sid), 0, "devices/%s/trades", sid)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Shop](res)
}

func Trade(rid, tid string) (*models.Shop, error) {
	res, err := Post("devices/%s/trades/%s", nil, rid, tid)
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Shop](res)
}

// Aliens
func Species() (*models.Species, error) {
	res, err := cacheGET("", 0, "species")
	if err != nil {
		return nil, err
	}
	return models.Parse[models.Species](res)
}

func ReloadStars() (string, error) {
	var out []string
	log := func(tmpl string, args ...any) {
		out = append(out, fmt.Sprintf(tmpl, args...))
	}
	res := func() string {
		return strings.Join(out, "\n")
	}
	// Get the current list of stars.
	seen := make(map[models.LocationID]*models.Star)
	rows, err := db.DB.Query(`
		SELECT designation, name, est_planets, spectral_type, has_life,
			position_x, position_y, position_z, has_hub, entry_point
		FROM stars`)
	if err != nil {
		return res(), err
	}
	old := 0
	for rows.Next() {
		old++
		s := new(models.Star)
		var x, y, z float32
		err := rows.Scan(&s.Designation, &s.Name, &s.EstimatedPlanets,
			&s.SpectralType, &s.HasLife, &x, &y, &z, &s.HasHub, &s.EntryPoint)
		if err != nil {
			return res(), err
		}
		s.Position = models.NewPosition(x, y, z)
		seen[s.Designation] = s
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	log("Loaded %d stars from cache", old)

	var added, updated []*models.Star
	updatedStar := func(a, b *models.Star) bool {
		if a.Designation != b.Designation {
			log("Comparing different stars %q and %q", a.Designation, b.Designation)
			return true
		}
		if a.Name != b.Name {
			log("%q: name updated: %q -> %q", a.Designation, a.Name, b.Name)
			return true
		}
		if a.Explored != b.Explored {
			log("%q: explore updated: %v -> %v", a.Designation, a.Explored, b.Explored)
			return true
		}
		if a.EstimatedPlanets != b.EstimatedPlanets {
			log("%q: est planets updated: %d -> %d", a.Designation, a.EstimatedPlanets, b.EstimatedPlanets)
			return true
		}
		if a.HasLife != b.HasLife {
			log("%q: has life updated: %v -> %v", a.Designation, a.HasLife, b.HasLife)
			return true
		}
		if a.SpectralType != b.SpectralType {
			log("%q: spectral type updated: %q -> %q", a.Designation, a.SpectralType, b.SpectralType)
			return true
		}
		return false
	}
	unchanged := 0
	accounted := make(map[models.LocationID]bool)
	catalog, err := StarCatalog()
	if err != nil {
		return res(), err
	}
	for _, star := range catalog.Stars {
		accounted[star.Designation] = true
		if old, ok := seen[star.Designation]; ok {
			if !updatedStar(old, star) {
				unchanged++
				continue
			}
			updated = append(updated, star)
		} else {
			added = append(added, star)
		}
		seen[star.Designation] = star
	}
	var missing []string
	for star := range accounted {
		if _, ok := seen[star]; ok {
			continue
		}
		missing = append(missing, string(star))
	}
	log(
		"Fetch done: %d total stars, %d added, %d updated, %d unchanged, %d removed",
		len(added)+len(updated)+unchanged, len(added), len(updated), unchanged, len(missing))
	for _, s := range append(added, updated...) {
		if err := s.Cache(); err != nil {
			return res(), err
		}
	}
	log("Updated done.")
	return res(), nil
}

func ProspectLogs(id *models.CodeAlias) (string, error) {
	logs, err := DeviceLogs(id, true, 0, 100)
	if err != nil {
		return "", err
	}
	var added []string
	for _, l := range logs.Events {
		if l.EventType != "prospect_complete" {
			continue
		}
		log("Prospect log: %s", l.Message)
		data, ok := l.Payload["stars"]
		if !ok {
			log("No stars in payload: %+v", l.Payload)
			continue
		}
		sList, ok := data.([]any)
		if !ok {
			log("Can't parse payload: %+v", data)
			continue
		}

		// {
		//  "coordinates": {
		//    "x": 101.3734,
		//    "y": -137.8722,
		//    "z": -86.3028
		//  },
		//  "designation": "QUADORANS"
		// }

		for _, s := range sList {
			m, ok := s.(map[string]any)
			if !ok {
				log("Can't map: %+v", s)
				continue
			}
			star := &models.Star{Designation: m["designation"].(models.LocationID)}
			coords := m["coordinates"].(map[string]any)
			star.Position = models.NewPosition(
				float32(coords["x"].(float64)),
				float32(coords["y"].(float64)),
				float32(coords["z"].(float64)),
			)
			if err := star.Cache(); err != nil {
				log("Error saving %s: %v", star.Designation, err)
			}
			added = append(added, string(star.Designation))
		}

	}
	return fmt.Sprintf("%d new stars added: %v", len(added), added), nil
}

func Contribute(id string, devs []*models.CodeAlias) (*models.Contribute, error) {
	ids := make([]string, len(devs))
	for n, d := range devs {
		ids[n] = d.String()
	}
	data, err := json.Marshal(map[string][]string{
		"devices": ids,
	})
	if err != nil {
		return nil, err
	}
	res, err := Post("locations/%s/contribute", data, id)
	return models.Parse[models.Contribute](res)
}
