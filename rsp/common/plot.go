package common

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

type PlotCfg struct {
	Debug       bool
	Hop         float32
	UseStation  bool
	UseHub      bool
	Recalculate bool
	Partial     bool
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
		cfg = &PlotCfg{Hop: 7.5, Partial: true}
	}
	starSrc, err := models.NewStar(src)
	if err != nil {
		return nil, err
	}
	sPos := starSrc.Position
	src = starSrc.Designation.Star()

	var dPos *models.Position
	if strings.ContainsAny(dst, ",:") {
		pos, err := models.ParsePosition(dst)
		if err != nil {
			return nil, err
		}
		Log("Plotting to arbitrary position %s", pos)
		if db == nil || db.DB == nil {
			return nil, fmt.Errorf("Can't find nearest star: not connected to cache")
		}
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
		dst = nStar.Designation.Star()
		if src == dst {
			return nil, fmt.Errorf("Nil route: %s->%s", src, dst)
		}
	} else {
		starDst, err := models.NewStar(dst)
		if err != nil {
			return nil, err
		}
		dPos = starDst.Position
	}

	origDist := sPos.Distance(dPos)
	if origDist == 0 {
		return nil, fmt.Errorf("No journey to take")
	}
	Log("Total distance: %.2fly", origDist)
	j := &models.Journey{
		Source:     src,
		Dest:       dst,
		MaxHop:     cfg.Hop,
		UseStation: cfg.UseStation,
		UseHub:     cfg.UseHub,
	}
	if err := j.Get(); !cfg.Recalculate && err == nil {
		Log("Loading cached route from %s:", j.Calculated.Format(time.Stamp))
		return j, nil
	}
	// If we don't have a route, and we're not explicitly recalculating, see if
	// we can reuse an existing route.
	if cfg.Partial {
		// See if there's a route that includes both starting and ending point
		j, err := GetPartialJourney(j)
		if err == nil && len(j.Legs) > 0 {
			return j, nil
		}
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
		var nextStationQueue []string
		for _, s := range queue {
			cnt++
			if time.Since(ts) > time.Second {
				Log("... Examined %d stars, %d in the queue, current best %v", cnt, len(queue), best)
				ts = time.Now()
			}

			debug("=== %s", s)
			// Get possible next steps using cached waypoint position if available
			sPos := waypoints[s].ToPosition
			if sPos == nil {
				qStar, err := models.NewStar(s)
				if err != nil {
					return nil, fmt.Errorf("Can't get star %q: %v", s, err)
				}
				sPos = qStar.Position
				waypoints[s].ToPosition = sPos
			}

			stars, err := TripStepCandidate(s, sPos, dPos, 0, cfg.Hop)
			if err != nil {
				return nil, fmt.Errorf("No candidates found from %v to %v: %v", s, dst, err)
			}
			if cfg.UseStation {
				extra, err := TripStepCandidate(s, sPos, dPos, cfg.Hop, 10)
				if err != nil {
					Log("No additional station candidates found from %v to %v: %v", s, dst, err)
				} else {
					stars = append(stars, extra...)
				}
			}
			if cfg.UseHub {
				extra, err := TripStepCandidate(s, sPos, dPos, cfg.Hop, 15)
				if err != nil {
					Log("No additional hub candidates found from %v to %v: %v", s, dst, err)
				} else {
					stars = append(stars, extra...)
				}
			}
			debug("%d candidates found", len(stars))
			for _, next := range stars {
				if best == nil || next.DistToDest < best.DistToDest {
					best = next
				}
				debug("  - %v", next)
				hopDist := next.DistFromSrc
				next.DistFromSrc += waypoints[s].DistFromSrc
				next.Step = waypoints[s].Step + 1
				debug("      total distance from src: %.2f", next.DistFromSrc)
				ex, ok := waypoints[next.To]
				if !ok {
					// New waypoint, add it to the queue and move on
					waypoints[next.To] = next
					// If it's an extended hop, add it to the less-preferred queue
					if hopDist > cfg.Hop {
						nextStationQueue = append(nextStationQueue, next.To)
						debug("      New extended waypoint: %s -> %s (hop: %.2f, behind: %.2f, ahead: %.2f)",
							next.From, next.To, hopDist, next.DistFromSrc, next.DistToDest)
					} else {
						nextQueue = append(nextQueue, next.To)
						debug("      New waypoint: %s -> %s (hop: %.2f, behind: %.2f, ahead: %.2f)",
							next.From, next.To, hopDist, next.DistFromSrc, next.DistToDest)
					}
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
				if waypoints[cur].From == src {
					break
				}
				cur = waypoints[cur].From
			}
			slices.Reverse(j.Legs)
			err := j.Cache()

			return j, err
		}

		if len(nextQueue) == 0 {
			if len(nextStationQueue) == 0 {
				break
			}
			queue = nextStationQueue
			continue
		}
		queue = nextQueue
	}
	Log("Failed to find route, closest is %v", best)

	return j, fmt.Errorf("Failed to find route, closest is %v", best)
}

func TripStepCandidate(start string, src, dst *models.Position, min_radius, max_radius float32) ([]*models.JourneyLeg, error) {
	if db == nil || db.DB == nil {
		return nil, fmt.Errorf("Not connected to cache")
	}
	rows, err := db.DB.Query(`
		SELECT designation, position, from_src, from_dst
		FROM (
			SELECT designation, position,
				position<->$1::cube AS from_src,
				position<->$2::cube AS from_dst
			FROM stars
		) sub
		WHERE from_src <= $3 AND from_src > $4 + 0.001;`,
		src.AsCube(), dst.AsCube(),
		max_radius, min_radius,
	)
	if err != nil {
		return nil, err
	}

	var res []*models.JourneyLeg
	var errs []error
	for rows.Next() {
		var desg string
		var fSrc, fDst float32
		var p cache.Position
		errs = append(errs, rows.Scan(&desg, &p, &fSrc, &fDst))
		res = append(res, &models.JourneyLeg{
			From: start,
			FromPosition: models.NewPosition(
				src.X,
				src.Y,
				src.Z),
			To:          desg,
			ToPosition:  models.ParseCube(p),
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
			continue
		}
		star := h.Location.Star()
		locs[star] = h.Code.Alias()
		if db != nil && db.DB != nil {
			if _, err := db.DB.Exec(`UPDATE stars SET has_my_hub=true WHERE designation = $1`, star); err != nil {
				return "", "", 0, fmt.Errorf("Can't update %s with hub: %v", star, err)
			}
		}
	}

	s, err := models.NewStar(star)
	if err != nil {
		return "", "", 0, err
	}
	if db == nil || db.DB == nil {
		return "", "", 0, fmt.Errorf("Not connected to cache")
	}
	nearest, dist, err := db.FindNearestHub(s.Position.X, s.Position.Y, s.Position.Z)
	if err != nil {
		return "", "", 0, fmt.Errorf("Can't find nearest hub: %v", err)
	}
	return locs[nearest], nearest, dist, nil
}

func GetPartialJourney(j *models.Journey) (*models.Journey, error) {
	if db == nil || db.DB == nil {
		return j, fmt.Errorf("Not connected to cache")
	}
	src := j.Source
	dst := j.Dest
	row := db.DB.QueryRow(`
			SELECT cached_journey_steps.journey_id FROM cached_journey_steps
			JOIN cached_journey ON cached_journey.id = cached_journey_steps.journey_id
			WHERE (src = $1 OR dest = $1)
			  AND max_hop <= $3
			  AND ($4 OR use_station = false)
			  AND ($5 OR use_hub = false)
			INTERSECT
			SELECT cached_journey_steps.journey_id FROM cached_journey_steps
			JOIN cached_journey ON cached_journey.id = cached_journey_steps.journey_id
			WHERE (src = $2 OR dest = $2)
			  AND max_hop <= $3
			  AND ($4 OR use_station = false)
			  AND ($5 OR use_hub = false)`, src, dst, j.MaxHop, j.UseStation, j.UseHub)
	var jid int
	if err := row.Scan(&jid); err != nil {
		Log("Can't find a partial journey (%s-%s): %v", src, dst, err)
		return j, nil
	}
	Log("Fount partial route from %s to %s in JID %d", src, dst, jid)
	rows, err := db.DB.Query(`
			SELECT src, dest, dist_src, dist_dest, step
			FROM cached_journey_steps
			WHERE journey_id = $1
			ORDER BY step`, jid)
	if err != nil {
		return j, fmt.Errorf("Can't get partial journey: %v", err)
	}
	defer rows.Close()
	var started bool
	for rows.Next() {
		l := new(models.JourneyLeg)
		if err := rows.Scan(&l.From, &l.To, &l.DistFromSrc, &l.DistToDest, &l.Step); err != nil {
			return j, fmt.Errorf("Can't load step: %v", err)
		}
		if l.From == src || l.From == dst {
			started = true
		}
		if !started {
			continue
		}
		j.Legs = append(j.Legs, l)
		if l.To == src || l.To == dst {
			break
		}
	}
	if len(j.Legs) == 0 {
		return j, fmt.Errorf("No useful journey extracted")
	}
	if err := rows.Err(); err != nil {
		return j, fmt.Errorf("Error scanning partial journey: %v", err)
	}
	if j.Legs[0].From != src {
		slices.Reverse(j.Legs)
		for i := range j.Legs {
			j.Legs[i].From, j.Legs[i].To = j.Legs[i].To, j.Legs[i].From
			j.Legs[i].DistFromSrc, j.Legs[i].DistToDest = j.Legs[i].DistToDest, j.Legs[i].DistFromSrc
		}
	}
	for i := range j.Legs {
		j.Legs[i].Step = i + 1
	}
	Log("Using route extracted from JID: %d", jid)
	return j, nil
}

func Distance(src, dst string) (float32, error) {
	// Get the position of a star, or a device
	getPos := func(o string) (*models.Position, error) {
		if s, err := models.NewStar(models.LocationID(o).Star()); err == nil {
			return s.Position, nil
		}
		ca := models.NewCodeAlias(o)
		if ca.Type() == "r" {
			res, err := rest.Replicant(ca)
			if err != nil {
				return nil, err
			}
			if res.HostedDeviceCode != nil {
				ca = res.HostedDeviceCode
			} else {
				return nil, fmt.Errorf("Can't find hosting device for %s", ca)
			}
		}
		dev, err := rest.DeviceInfo(ca)
		if err != nil {
			return nil, err
		}
		return dev.GetPosition(), nil
	}

	if src == "" || dst == "" {
		return 0, fmt.Errorf("Can't get distance to nowhere (%q->%q)", src, dst)
	}

	posA, err := getPos(src)
	if err != nil {
		return 0, err
	}
	posB, err := getPos(dst)
	if err != nil {
		return 0, err
	}

	return posA.Distance(posB), nil
}

func NearestRelay(dest string) (string, error) {
	// Get the home relay network
	net, err := rest.DeviceNetwork(models.NewCodeAlias("sh-1"))
	if err != nil {
		return "", err
	}
	star, err := models.NewStar(dest)
	if err != nil {
		return "", err
	}
	var closest float32
	var relay string
	for _, r := range net.Connections {
		rStar, err := models.NewStar(r.Star)
		if err != nil {
			return "", err
		}
		dist := star.Position.Distance(rStar.Position)
		if dist == 0 {
			return dest, nil
		}
		if closest == 0 || dist < closest {
			closest = dist
			relay = rStar.Designation.Star()
		}
	}
	return relay, nil
}

func RelayIsland(loc string, hop float32, limit int) ([]string, error) {
	// Load the main relay network, so we know when we're connected
	// Check if the starting point is in the network
	// Check if all the neighbours within a hop are in the network
	// Repeat
	// Once identified, return the island if it's not connected, an error if it is

	net, err := rest.DeviceNetwork(models.NewCodeAlias("sh-1"))
	if err != nil {
		return nil, err
	}
	inNet := make(map[string]bool)
	inNet["MENKUNT"] = true
	for _, c := range net.Connections {
		inNet[c.Star] = true
	}

	if inNet[loc] {
		return nil, fmt.Errorf("%q is already in the relay network", loc)
	}

	island := map[string]string{loc: "-"}
	getPath := func(s string) ([]string, error) {
		path := []string{s}
		for {
			r, ok := island[s]
			if !ok {
				break
			}
			if r == "-" {
				return path, nil
			}
			path = append([]string{r}, path...)
			s = r
		}
		return path, fmt.Errorf("%q isn't on the island: %v", s, path)
	}
	var queue []string
	for {
		s, err := models.NewStar(loc)
		if err != nil {
			return nil, err
		}
		next, err := db.QueryStarsInRadius(s.Position.X, s.Position.Y, s.Position.Z, hop, 0)
		if err != nil {
			return nil, err
		}
		for _, n := range next {
			nd := n.Designation
			if _, ok := island[nd]; ok {
				continue
			}
			island[nd] = loc
			if inNet[nd] {
				Log("Found path via %q", nd)
				path, err := getPath(nd)
				if err != nil {
					return nil, err
				}
				return nil, fmt.Errorf("%q is reachable from the relay network:\n%s", path[0], strings.Join(path, " -> "))
			}
			queue = append(queue, nd)
		}
		if len(queue) == 0 {
			break
		}
		if limit > 0 && len(island) > limit {
			return nil, fmt.Errorf("Island exceeds %d limit", limit)
		}
		loc, queue = queue[0], queue[1:]
	}
	var res []string
	for k := range island {
		res = append(res, k)
	}
	return res, nil
}
