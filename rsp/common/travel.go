package common

import (
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

type etaCache struct {
	via []string
	eta time.Duration
}

func getCachedTrip(vessel, from, to string) *etaCache {
	if db == nil {
		return nil
	}
	// Make sure the state of the galaxy hasn't invalidated our cache.
	cksum, err := db.ChecksumHubs()
	if err != nil {
		Log("Error getting hub state: %v", err)
		return nil
	}
	_, err = db.Exec("DELETE FROM eta_cache WHERE hub_checksum != $1", cksum)
	if err != nil {
		Log("Error clearing cache: %v", err)
		return nil
	}
	row := db.QueryRow(`
		SELECT eta, via
		FROM eta_cache
		WHERE vessel=$1 AND origin=$2 AND destination=$3
	`, vessel, from, to)
	if row.Err() != nil {
		Log("Error querying cache: %v", err)
		return nil
	}
	var eta string
	var via pq.StringArray
	if err := row.Scan(&eta, &via); err != nil {
		if !strings.Contains(err.Error(), "no rows in result set") {
			Log("Error scanning result: %v", err)
		}
		return nil
	}
	dt, err := cache.PsqlDuration(eta)
	if err != nil {
		Log("Invalid duration %q: %v", eta, err)
		return nil
	}
	return &etaCache{
		via: via,
		eta: dt,
	}
}

func Travel(id *models.CodeAlias, loc string, dryRun bool, via ...string) (time.Time, error) {
	location := models.LocationID(loc)
	var eta time.Time
	info, err := rest.CachedDeviceInfo(id, !dryRun)
	if err != nil {
		return eta, fmt.Errorf("Can't get %s info: %v", id.Alias(), err)
	}
	if info.Location == location {
		Log("%s is already at %s", id.Alias(), loc)
		return eta, nil
	}
	star, err := models.NewStar(loc)
	if err != nil {
		return eta, fmt.Errorf("Can't load %s: %v", loc, err)
	}
	if string(location) == location.Star() {
		if info.Location == star.EntryPoint {
			Log("%s is already at the entry point of %s", id.Alias(), loc)
			return eta, nil
		} else if star.EntryPoint != "" {
			Log("Setting destination to entry point %q", star.EntryPoint)
			location = star.EntryPoint
		}
	}
	if info.Location.Star() == location.Star() {
		Log("%s is already in system", id.Alias())
		via = []string{"-"}
	}
	cfg := map[string]any{
		"destination": location,
	}
	Log("Plotting travel: %s -> %s (%.2fly)",
		info.Location.Star(), location.Star(),
		info.GetPosition().Distance(star.Position))

	cachedTrip := getCachedTrip(info.Type, info.Location.Star(), string(location))
	if len(via) == 0 && cachedTrip != nil {
		// See if we have this route already cached
		Log("Using cached plan: %s (%s)", time.Now().Add(cachedTrip.eta), cachedTrip.eta)
		if len(cachedTrip.via) > 0 {
			cfg["via"] = cachedTrip.via
		}
	} else {
		// If not, figure out the best route.
		cfg, err = getBestRoute(info, location, via)
		if err != nil {
			return eta, err
		}
	}
	cfg["dry_run"] = dryRun
	res, err := rest.DeviceCommand[models.CommandResp](id, "travel", cfg)
	if err != nil {
		return eta, fmt.Errorf("Failed to send %s from %q to %q: %v",
			id.Alias(), info.Location, location, err)
	}
	eta = time.Now().Add(res.TotalTime.Duration())
	if dryRun {
		Log("[DRYRUN] Shipped %s to %s: ETA %s (%s)", id, location, eta, res.TotalTime.Duration())
	} else {
		Log("Shipped %s to %s: ETA %s (%s)", id, location, eta, res.TotalTime.Duration())
	}
	return eta, nil
}

func getBestRoute(info *models.Device, to models.LocationID, via []string) (map[string]any, error) {
	cfg := map[string]any{
		"destination": to,
		"dry_run":     true,
	}
	// Allow forcing direct travel, if we need that for some reason
	if len(via) == 1 && via[0] == "-" {
		Log("Direct travel requested, not applying auto-route")
		cfg["via"] = "direct"
		return cfg, nil
	} else if len(via) > 0 {
		Log("Using provided route: %v", via)
		cfg["via"] = via
		return cfg, nil
	}
	type opt struct {
		t time.Duration
		v any
	}
	opts := make(map[string]opt)
	Log("Auto-calculating route")

	// Find the nearest hub
	_, star, dist, err := NearestHub(to.Star())
	if err != nil {
		return nil, fmt.Errorf("Can't find nearest hub to %q: %v", to.Star(), err)
	}
	Log("Nearest hub: %s (%.2f ly from %s)", star, dist, to.Star())

	// Check the route via the auto routing chip
	res, err := rest.DeviceCommand[models.CommandResp](info.Code, "travel", cfg)
	if err != nil {
		Log("Auto-route failed: %v", err)
		if strings.Contains(err.Error(), "Device is out of comms range") {
			return nil, err
		}
	} else {
		opts["auto"] = opt{
			t: res.TotalTime.Duration(),
			v: nil,
		}
	}

	// Check the route "direct" (but only if we're going to the system edge)
	if string(to) == to.Star() {
		cfg["via"] = "direct"
	} else {
		cfg["via"] = []string{to.Star()}
	}
	res, err = rest.DeviceCommand[models.CommandResp](info.Code, "travel", cfg)
	if err != nil {
		Log("Direct-route failed: %v", err)
	} else {
		opts["direct"] = opt{
			t: res.TotalTime.Duration(),
			v: cfg["via"],
		}
	}

	// Now check via the nearest hub
	// - If its at our destination system, no need to 'via'
	if star != to.Star() {
		via = append(via, star)
	}
	// - Are we going to the system edge, or to a particular object?
	if string(to) != to.Star() {
		via = append(via, to.Star())
	}
	if len(via) > 0 {
		cfg["via"] = via
	}

	res, err = rest.DeviceCommand[models.CommandResp](info.Code, "travel", cfg)
	if err != nil {
		Log("Hub-route failed: %v", err)
	} else {
		opts["hub"] = opt{
			t: res.TotalTime.Duration(),
			v: cfg["via"],
		}
	}

	// Now compare our options
	var best = "auto"
	for _, t := range []string{"auto", "direct", "hub"} {
		opt, ok := opts[t]
		if !ok {
			continue
		}
		Log("Routing mode: %s: %v %s", t, opt.v, opt.t)
		if opts[t].t > 0 && opt.t < opts[best].t {
			Log("Faster by %v", opts[best].t-opt.t)
			best = t
		}
	}
	if opts[best].v == nil {
		delete(cfg, "via")
	} else {
		cfg["via"] = opts[best].v
	}

	cksum, err := db.ChecksumHubs()
	if err != nil {
		Log("Error getting hub state: %v", err)
		return cfg, nil
	}
	if err := db.Update(cache.ETACacheTable, map[string]any{
		"hub_checksum": cksum,
		"vessel":       info.Type,
		"origin":       info.Location.Star(),
		"destination":  to,
		"eta":          opts[best].t.Seconds(),
		"via":          opts[best].v,
	}); err != nil {
		Log("Error caching ETA: %v", err)
	}

	return cfg, nil
}
