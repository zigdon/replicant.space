package common

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

type PlotCfg struct {
	Debug       bool
	Hop         float32
	Recalculate bool
}

func PlotTrip(src, dst string, cfg *PlotCfg) (*models.Journey, error) {
	// Pathfinding:
	// Keep a list of waypoints
	//   - previous hop to get there
	//   - distance travelled from origin to get there
	//   - remaining distance to destination (as-crow-spaceflies)
	// Loop over waypoints, sorted by lowest travelled + remaining distance
	// For each waypoint, get the possible next steps
	// Ignore repeats (unless this is a shorter way to get to them)
	// Repeat until destination is found

	if cfg == nil {
		cfg = &PlotCfg{Hop: 7.5}
	}
	starSrc, err := models.NewStar(src)
	if err != nil {
		return nil, err
	}
	sPos := starSrc.Position

	var dPos *models.Position
	if strings.Contains(dst, ",") {
		pos, err := models.ParsePosition(dst)
		if err != nil {
			return nil, err
		}
		Log("Plotting to arbitrary position %s", pos)
		nearest, dist, err := db.FindNearestStar(pos.X, pos.Y, pos.Z)
		if err != nil {
			return nil, fmt.Errorf("Can't find nearest star: %v", err)
		}
		Log("Nearest star: %s (%.2fly away)", nearest, dist)
		nStar, err := models.NewStar(nearest)
		if err != nil {
			return nil, err
		}
		dPos = nStar.Position
	} else {
		starDst, err := models.NewStar(dst)
		if err != nil {
			return nil, err
		}
		dPos = starDst.Position
	}

	origDist := sPos.Distance(dPos)
	Log("Total distance: %.2fly", origDist)
	j := &models.Journey{
		Source: src,
		Dest:   dst,
		MaxHop: cfg.Hop,
	}
	if err := j.Get(); !cfg.Recalculate && err == nil {
		Log("Loading cached route from %s:", j.Calculated.Format(time.Stamp))
		return j, nil
	}
	// We're going to recalculate the legs, nuke what we already had.
	j.Legs = j.Legs[:0]

	waypoints := map[string]*models.JourneyLeg{
		src: {
			To:         src,
			ToPosition: sPos,
			DistToDest: origDist,
		},
	}

	queue := []string{src}
	debug := func(tmpl string, args ...any) {
		if !cfg.Debug {
			return
		}
		Log(tmpl, args...)
	}
	var best *models.JourneyLeg
	ts := time.Now()
	var cnt int
	for {
		// Sort by distance travelled + left to go
		slices.SortFunc(queue, func(a, b string) int {
			return cmp.Compare(
				waypoints[a].DistFromSrc+waypoints[a].DistToDest,
				waypoints[b].DistFromSrc+waypoints[b].DistToDest)
		})
		debug("starting iteration over queue: %v", queue)
		var nextQueue []string
		for _, s := range queue {
			cnt++
			if time.Since(ts) > time.Second {
				Log("... Examined %d stars, %d in the queue, current best %v", cnt, len(queue), best)
				ts = time.Now()
			}

			debug("=== %s", s)
			// Get possible next steps
			qStar, err := models.NewStar(s)
			if err != nil {
				return nil, fmt.Errorf("Can't get star %q: %v", s, err)
			}
			stars, err := TripStepCandidate(s, qStar.Position, dPos, cfg.Hop)
			if err != nil {
				return nil, fmt.Errorf("No candidates found from %v to %v: %v", s, dst, err)
			}
			debug("%d candidates found", len(stars))
			for _, next := range stars {
				if best == nil || next.DistToDest < best.DistToDest {
					best = next
				}
				debug("  - %v", next)
				next.DistFromSrc += waypoints[s].DistFromSrc
				next.Step = waypoints[s].Step + 1
				debug("      total distance from src: %.2f", next.DistFromSrc)
				ex, ok := waypoints[next.To]
				if !ok {
					// New waypoint, add it to the queue and move on
					waypoints[next.To] = next
					nextQueue = append(nextQueue, next.To)
					debug("      New waypoint: %s -> %s (behind: %.2f, ahead: %.2f)",
						next.From, next.To, next.DistFromSrc, next.DistToDest)
					continue
				}
				// Existing waypoint, if it's a shorter path to get there, update it.
				if ex.DistFromSrc > next.DistFromSrc {
					debug("      Shorter path to %q, from %q (%.2f) rather than %q (%.2f)",
						next.To, next.From, next.DistFromSrc, ex.From, ex.DistFromSrc)
					ex.DistFromSrc = next.DistFromSrc
					ex.From = next.From
					ex.FromPosition = next.FromPosition
					ex.Step = next.Step
					waypoints[next.To] = ex
					continue
				}
				debug("      Discarding longer leg")
			}
		}

		// Find the current best route
		cur := src
		closest := waypoints[src].DistToDest
		for k, v := range waypoints {
			if v.DistToDest < closest {
				cur = k
				closest = v.DistToDest
			}
		}

		if waypoints[cur].To == dst {
			for {
				j.Legs = append(j.Legs, waypoints[cur])
				if cur == src {
					break
				}
				cur = waypoints[cur].From
			}

			return j, j.Cache()
		}

		if len(nextQueue) == 0 {
			break
		}
		queue = nextQueue
	}
	Log("Failed to find route, closest is %v", best)

	return j, fmt.Errorf("Failed to find route, closest is %v", best)
}

func TripStepCandidate(start string, src, dst *models.Position, radius float32) ([]*models.JourneyLeg, error) {
	rows, err := db.DB.Query(`
		SELECT designation, position_x, position_y, position_z, from_src, from_dst
		FROM (
			SELECT designation, position_x, position_y, position_z,
				sqrt(
					power(position_x - $1, 2) + 
					power(position_y - $2, 2) + 
					power(position_z - $3, 2)
				) AS from_src,
				sqrt(
					power(position_x - $4, 2) +
					power(position_y - $5, 2) + 
					power(position_z - $6, 2)
				) AS from_dst
			FROM stars
		) sub
		WHERE from_src <= $7 AND from_src > 0.001;`,
		src.X, src.Y, src.Z,
		dst.X, dst.Y, dst.Z,
		radius,
	)
	if err != nil {
		return nil, err
	}

	var res []*models.JourneyLeg
	var errs []error
	for rows.Next() {
		var desg string
		var x, y, z, fSrc, fDst float32
		errs = append(errs, rows.Scan(&desg, &x, &y, &z, &fSrc, &fDst))
		res = append(res, &models.JourneyLeg{
			From: start,
			FromPosition: models.NewPosition(
				src.X,
				src.Y,
				src.Z),
			To:          desg,
			ToPosition:  models.NewPosition(x, y, z),
			DistFromSrc: fSrc,
			DistToDest:  fDst,
		},
		)
	}
	errs = append(errs, rows.Err())

	return res, errors.Join(errs...)
}

func NearestHub(star string) (string, string, float32, error) {
	// Update the db with our hubs
	hubs, err := rest.RefreshDevices(map[string]string{
		"device_type": "system_hub",
	})
	locs := make(map[string]string)
	for _, h := range hubs {
		if h.Status != "relaying" {
			Log("Ignoring inactive hub %s at %s", h.Code.Alias(), h.Location)
			continue
		}
		star := h.Location.Star()
		locs[star] = h.Code.Alias()
		if _, err := db.DB.Exec(`UPDATE stars SET has_my_hub=true WHERE designation = $1`, star); err != nil {
			return "", "", 0, fmt.Errorf("Can't update %s with hub: %v", star, err)
		}
	}

	s, err := models.NewStar(star)
	if err != nil {
		return "", "", 0, err
	}
	nearest, dist, err := db.FindNearestHub(s.Position.X, s.Position.Y, s.Position.Z)
	if err != nil {
		return "", "", 0, fmt.Errorf("Can't find nearest hub: %v", err)
	}
	return locs[nearest], nearest, dist, nil
}
