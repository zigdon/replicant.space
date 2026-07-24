package common

import (
	"fmt"
	"time"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

func Travel(id *models.CodeAlias, loc string, dryRun bool, via ...string) (time.Time, error) {
	location := models.LocationID(loc)
	var eta time.Time
	info, err := rest.RefreshDeviceInfo(id)
	if err != nil {
		return eta, fmt.Errorf("Can't get %s info: %v", id.Alias(), err)
	}
	if info.Location == location {
		Log("%s is already at %s", id.Alias(), loc)
		return eta, nil
	}
	cfg := map[string]any{
		"destination": location,
	}
	// Allow forcing direct travel, if we need that for some reason
	if len(via) == 1 && via[0] == "-" {
		Log("Direct travel requested, not applying auto-route")
		cfg["via"] = "direct"
	} else if len(via) > 0 {
		Log("Using provided route: %v", via)
	} else {
		type opt struct {
			t time.Duration
			v any
		}
		opts := make(map[string]opt)
		Log("Auto-calculating route")
		// Find the nearest hub
		_, star, dist, err := NearestHub(location.Star())
		if err != nil {
			return eta, fmt.Errorf("Can't find nearest hub to %q: %v", location.Star(), err)
		}
		Log("Nearest hub: %s (%.2f ly from %s)", star, dist, location.Star())
		// Check the route via the auto routing chip
		cfg["dry_run"] = true
		res, err := rest.DeviceCommand[models.CommandResp](id, "travel", cfg)
		if err != nil {
			Log("Auto-route failed: %v", err)
		} else {
			opts["auto"] = opt{
				t: res.TotalTime.Duration(),
				v: nil,
			}
		}
		// Check the route "direct" (but only if we're going to the system edge)
		cfg["dry_run"] = true
		if string(location) == location.Star() {
			cfg["via"] = "direct"
		} else {
			cfg["via"] = []string{location.Star()}
		}
		res, err = rest.DeviceCommand[models.CommandResp](id, "travel", cfg)
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
		var via []string
		if star != location.Star() {
			via = append(via, star)
		}
		// - Are we going to the system edge, or to a particular object?
		if string(location) != location.Star() {
			via = append(via, location.Star())
		}
		if len(via) > 0 {
			cfg["via"] = via
		}

		res, err = rest.DeviceCommand[models.CommandResp](id, "travel", cfg)
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
	}
	cfg["dry_run"] = dryRun
	res, err := rest.DeviceCommand[models.CommandResp](id, "travel", cfg)
	if err != nil {
		return eta, fmt.Errorf("Failed to send %s from %q to %q: %v", id.Alias(), info.Location, location, err)
	}
	eta = res.Arrives.Time()
	Log("Shipped %s to %s: ETA %s", id.Alias(), location,
		res.TotalTime.Duration().Truncate(time.Second))
	return eta, nil
}
