package common

import (
	"container/heap"
	"errors"
	"fmt"
	"math"
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
	Banned      []string
}

// StarSpatialNode represents a star within the in-memory 3D spatial index.
type StarSpatialNode struct {
	Designation string
	Position    *models.Position
}

type voxelKey struct {
	X, Y, Z int
}

// SpatialStarGrid provides fast O(1) 3D spatial lookups for candidate neighbors,
// with lazy voxel paging to transparently load unexplored sectors from the database.
type SpatialStarGrid struct {
	CellSize float32
	grid     map[voxelKey][]*StarSpatialNode
	stars    map[string]*StarSpatialNode
	loaded   map[voxelKey]bool
	db       *cache.Cache
	banned   []string
	debugFn  func(tmpl string, args ...any)
}

func NewSpatialStarGrid(cellSize float32) *SpatialStarGrid {
	if cellSize <= 0 {
		cellSize = 7.5
	}
	return &SpatialStarGrid{
		CellSize: cellSize,
		grid:     make(map[voxelKey][]*StarSpatialNode),
		stars:    make(map[string]*StarSpatialNode),
		loaded:   make(map[voxelKey]bool),
	}
}

func (sg *SpatialStarGrid) SetLoader(db *cache.Cache, banned []string, debugFn func(tmpl string, args ...any)) {
	sg.db = db
	sg.banned = banned
	sg.debugFn = debugFn
}

func (sg *SpatialStarGrid) MarkBoxLoaded(minK, maxK voxelKey) {
	for x := minK.X; x <= maxK.X; x++ {
		for y := minK.Y; y <= maxK.Y; y++ {
			for z := minK.Z; z <= maxK.Z; z++ {
				sg.loaded[voxelKey{X: x, Y: y, Z: z}] = true
			}
		}
	}
}

func (sg *SpatialStarGrid) loadVoxel(k voxelKey) error {
	if sg.loaded[k] || sg.db == nil || sg.db.DB == nil {
		sg.loaded[k] = true
		return nil
	}
	sg.loaded[k] = true

	minX := float32(k.X) * sg.CellSize
	maxX := float32(k.X+1) * sg.CellSize
	minY := float32(k.Y) * sg.CellSize
	maxY := float32(k.Y+1) * sg.CellSize
	minZ := float32(k.Z) * sg.CellSize
	maxZ := float32(k.Z+1) * sg.CellSize

	records, err := sg.db.QueryStarsInBox(minX, minY, minZ, maxX, maxY, maxZ, 0)
	if err != nil {
		return err
	}
	var inserted int
	for _, r := range records {
		if slices.Contains(sg.banned, r.Designation) {
			if sg.debugFn != nil {
				sg.debugFn("Avoiding banned star %q", r.Designation)
			}
			continue
		}
		pos := models.ParseCube(r.Position)
		sg.Insert(r.Designation, pos)
		inserted++
	}
	if sg.debugFn != nil && inserted > 0 {
		sg.debugFn("Lazy-loaded voxel [%d,%d,%d]: %d stars (index total: %d)",
			k.X, k.Y, k.Z, inserted, sg.Count())
	}
	return nil
}

func (sg *SpatialStarGrid) keyFor(pos *models.Position) voxelKey {
	if pos == nil {
		return voxelKey{}
	}
	return voxelKey{
		X: int(math.Floor(float64(pos.X / sg.CellSize))),
		Y: int(math.Floor(float64(pos.Y / sg.CellSize))),
		Z: int(math.Floor(float64(pos.Z / sg.CellSize))),
	}
}

func (sg *SpatialStarGrid) Insert(desg string, pos *models.Position) {
	if pos == nil || desg == "" {
		return
	}
	if _, ok := sg.stars[desg]; ok {
		return
	}
	node := &StarSpatialNode{
		Designation: desg,
		Position:    pos,
	}
	sg.stars[desg] = node
	k := sg.keyFor(pos)
	sg.grid[k] = append(sg.grid[k], node)
}

func (sg *SpatialStarGrid) Get(desg string) *StarSpatialNode {
	return sg.stars[desg]
}

func (sg *SpatialStarGrid) Count() int {
	return len(sg.stars)
}

func (sg *SpatialStarGrid) FindNeighbors(pos *models.Position, minRadius, maxRadius float32) []*StarSpatialNode {
	if pos == nil || maxRadius <= 0 {
		return nil
	}
	cellRadius := int(math.Ceil(float64(maxRadius / sg.CellSize)))
	baseKey := sg.keyFor(pos)

	if sg.db != nil && sg.db.DB != nil {
		for dx := -cellRadius; dx <= cellRadius; dx++ {
			for dy := -cellRadius; dy <= cellRadius; dy++ {
				for dz := -cellRadius; dz <= cellRadius; dz++ {
					k := voxelKey{
						X: baseKey.X + dx,
						Y: baseKey.Y + dy,
						Z: baseKey.Z + dz,
					}
					if !sg.loaded[k] {
						_ = sg.loadVoxel(k)
					}
				}
			}
		}
	}

	minR2 := minRadius * minRadius
	maxR2 := maxRadius * maxRadius

	var res []*StarSpatialNode
	for dx := -cellRadius; dx <= cellRadius; dx++ {
		for dy := -cellRadius; dy <= cellRadius; dy++ {
			for dz := -cellRadius; dz <= cellRadius; dz++ {
				k := voxelKey{
					X: baseKey.X + dx,
					Y: baseKey.Y + dy,
					Z: baseKey.Z + dz,
				}
				nodes, ok := sg.grid[k]
				if !ok {
					continue
				}
				for _, n := range nodes {
					dxPos := pos.X - n.Position.X
					dyPos := pos.Y - n.Position.Y
					dzPos := pos.Z - n.Position.Z
					dist2 := dxPos*dxPos + dyPos*dyPos + dzPos*dzPos
					if dist2 <= maxR2 && dist2 > minR2+0.0001 {
						res = append(res, n)
					}
				}
			}
		}
	}
	return res
}

// Hop penalty constants for hierarchical routing (Relay -> Station -> Hub).
const (
	stationHopPenalty float32 = 1000.0
	hubHopPenalty     float32 = 100000.0
)

// aStarItem represents a search node in the priority queue.
type aStarItem struct {
	Star        string
	Position    *models.Position
	CostFromSrc float32 // g(n): penalized search cost from source
	DistFromSrc float32 // Actual physical light-years traveled from source
	DistToDest  float32 // h(n): heuristic to destination
	Priority    float32 // f(n) = CostFromSrc + DistToDest with tie-breaker
	From        string
	FromPos     *models.Position
	HopDist     float32
	Step        int // 1-based hop count from origin along this path
	Index       int // Internal index in the container/heap priority queue
}

type aStarPriorityQueue []*aStarItem

func (pq aStarPriorityQueue) Len() int { return len(pq) }
func (pq aStarPriorityQueue) Less(i, j int) bool {
	if pq[i].Priority == pq[j].Priority {
		return pq[i].DistToDest < pq[j].DistToDest
	}
	return pq[i].Priority < pq[j].Priority
}
func (pq aStarPriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}
func (pq *aStarPriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*aStarItem)
	item.Index = n
	*pq = append(*pq, item)
}
func (pq *aStarPriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.Index = -1
	*pq = old[0 : n-1]
	return item
}

func PlotTrip(src, dst string, cfg *PlotCfg) (*models.Journey, error) {
	if cfg == nil {
		cfg = &PlotCfg{Hop: 7.5, Partial: true}
	}
	if cfg.Hop == 0 {
		cfg.Hop = 7.5
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
		Log("Loading cached route #%d from %s:", j.ID, j.Calculated.Format(time.Stamp))
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

	debug := func(tmpl string, args ...any) {
		if !cfg.Debug {
			return
		}
		Log(tmpl, args...)
	}

	// Determine max jump radius across standard, station, and hub jumps
	maxHop := cfg.Hop
	if cfg.UseStation && 10.0 > maxHop {
		maxHop = 10.0
	}
	if cfg.UseHub && 15.0 > maxHop {
		maxHop = 15.0
	}

	// Pre-fetch sector stars into spatial grid snapped to voxel boundaries
	padding := float32(15.0)
	minPos := models.NewPosition(
		float32(math.Min(float64(sPos.X), float64(dPos.X)))-padding,
		float32(math.Min(float64(sPos.Y), float64(dPos.Y)))-padding,
		float32(math.Min(float64(sPos.Z), float64(dPos.Z)))-padding,
	)
	maxPos := models.NewPosition(
		float32(math.Max(float64(sPos.X), float64(dPos.X)))+padding,
		float32(math.Max(float64(sPos.Y), float64(dPos.Y)))+padding,
		float32(math.Max(float64(sPos.Z), float64(dPos.Z)))+padding,
	)

	sg := NewSpatialStarGrid(maxHop)
	sg.SetLoader(db, cfg.Banned, debug)

	minVoxel := sg.keyFor(minPos)
	maxVoxel := sg.keyFor(maxPos)

	boxMinX := float32(minVoxel.X) * sg.CellSize
	boxMaxX := float32(maxVoxel.X+1) * sg.CellSize
	boxMinY := float32(minVoxel.Y) * sg.CellSize
	boxMaxY := float32(maxVoxel.Y+1) * sg.CellSize
	boxMinZ := float32(minVoxel.Z) * sg.CellSize
	boxMaxZ := float32(maxVoxel.Z+1) * sg.CellSize

	if db != nil && db.DB != nil {
		records, err := db.QueryStarsInBox(boxMinX, boxMinY, boxMinZ, boxMaxX, boxMaxY, boxMaxZ, 0)
		if err != nil {
			return nil, fmt.Errorf("Failed to query corridor stars: %v", err)
		}
		for _, r := range records {
			if slices.Contains(cfg.Banned, r.Designation) {
				debug("Avoiding banned star %q", r.Designation)
				continue
			}
			pos := models.ParseCube(r.Position)
			sg.Insert(r.Designation, pos)
		}
		sg.MarkBoxLoaded(minVoxel, maxVoxel)
	}
	sg.Insert(src, sPos)
	sg.Insert(dst, dPos)

	debug("Loaded %d stars into in-memory spatial index", sg.Count())

	// A* Priority Queue setup
	openSet := &aStarPriorityQueue{}
	heap.Init(openSet)

	gScore := make(map[string]float32)
	gScore[src] = 0

	cameFrom := make(map[string]*models.JourneyLeg)
	closedSet := make(map[string]bool)

	const eps float32 = 1e-4 // Tie-breaking factor towards destination

	heap.Push(openSet, &aStarItem{
		Star:        src,
		Position:    sPos,
		CostFromSrc: 0,
		DistFromSrc: 0,
		DistToDest:  origDist,
		Priority:    origDist,
		Step:        0,
	})

	var bestItem *aStarItem
	ts := time.Now()
	var cnt int

	for openSet.Len() > 0 {
		curr := heap.Pop(openSet).(*aStarItem)
		cnt++

		if closedSet[curr.Star] {
			continue
		}
		closedSet[curr.Star] = true

		if bestItem == nil || curr.DistToDest < bestItem.DistToDest {
			bestItem = curr
		}

		if time.Since(ts) > time.Second {
			Log("... Examined %d stars, %d in queue, current best %s (%.2fly left)", cnt, openSet.Len(), curr.Star, curr.DistToDest)
			ts = time.Now()
		}

		debug("=== %s (g: %.2f, h: %.2f, priority: %.2f)", curr.Star, curr.DistFromSrc, curr.DistToDest, curr.Priority)

		if curr.Star == dst {
			curStar := dst
			var usedStation, usedHub bool
			for {
				leg, ok := cameFrom[curStar]
				if !ok {
					break
				}
				hopDist := leg.FromPosition.Distance(leg.ToPosition)
				if hopDist > 10.0 {
					usedHub = true
					usedStation = true
				} else if hopDist > cfg.Hop {
					usedStation = true
				}
				j.Legs = append(j.Legs, leg)
				if leg.From == src {
					break
				}
				curStar = leg.From
			}
			slices.Reverse(j.Legs)
			var totDist float32
			for i := range j.Legs {
				hopDist := j.Legs[i].FromPosition.Distance(j.Legs[i].ToPosition)
				totDist += hopDist
				j.Legs[i].DistFromSrc = totDist
				j.Legs[i].DistToDest = j.Legs[i].ToPosition.Distance(dPos)
				j.Legs[i].Step = i + 1
			}
			j.UseStation = usedStation
			j.UseHub = usedHub
			err := j.Cache()
			return j, err
		}

		// Find candidate neighbors from spatial index
		neighbors := sg.FindNeighbors(curr.Position, 0, cfg.Hop)
		if cfg.UseStation && 10.0 > cfg.Hop {
			stationNeighbors := sg.FindNeighbors(curr.Position, cfg.Hop, 10.0)
			neighbors = append(neighbors, stationNeighbors...)
		}
		if cfg.UseHub && 15.0 > cfg.Hop {
			minR := cfg.Hop
			if cfg.UseStation && 10.0 > minR {
				minR = 10.0
			}
			hubNeighbors := sg.FindNeighbors(curr.Position, minR, 15.0)
			neighbors = append(neighbors, hubNeighbors...)
		}

		debug("  %d candidate neighbors found", len(neighbors))

		for _, nbr := range neighbors {
			if closedSet[nbr.Designation] {
				continue
			}

			hopDist := curr.Position.Distance(nbr.Position)
			hopCost := hopDist
			if hopDist > 10.0 {
				hopCost += hubHopPenalty
			} else if hopDist > cfg.Hop {
				if cfg.UseStation {
					hopCost += stationHopPenalty
				} else {
					hopCost += hubHopPenalty
				}
			}

			tentativeCost := curr.CostFromSrc + hopCost
			tentativeDist := curr.DistFromSrc + hopDist

			currentCost, visited := gScore[nbr.Designation]
			if !visited || tentativeCost < currentCost {
				gScore[nbr.Designation] = tentativeCost
				h := nbr.Position.Distance(dPos)
				f := tentativeCost + h
				priority := f*(1.0+eps) - h*eps

				cameFrom[nbr.Designation] = &models.JourneyLeg{
					From:         curr.Star,
					FromPosition: curr.Position,
					To:           nbr.Designation,
					ToPosition:   nbr.Position,
					DistFromSrc:  tentativeDist,
					DistToDest:   h,
					Step:         curr.Step + 1,
				}

				heap.Push(openSet, &aStarItem{
					Star:        nbr.Designation,
					Position:    nbr.Position,
					CostFromSrc: tentativeCost,
					DistFromSrc: tentativeDist,
					DistToDest:  h,
					Priority:    priority,
					From:        curr.Star,
					FromPos:     curr.Position,
					HopDist:     hopDist,
					Step:        curr.Step + 1,
				})

				debug("  - Queued %s -> %s (hop: %.2f, cost: %.2f, dist: %.2f, h: %.2f)", curr.Star, nbr.Designation, hopDist, tentativeCost, tentativeDist, h)
			}
		}
	}

	var closestDesc string
	if bestItem != nil {
		closestDesc = fmt.Sprintf("%s (%.2fly from src, %.2fly to dest)", bestItem.Star, bestItem.DistFromSrc, bestItem.DistToDest)
	} else {
		closestDesc = "none"
	}
	Log("Failed to find route, closest is %s", closestDesc)

	return j, fmt.Errorf("Failed to find route, closest is %s", closestDesc)
}

func TripStepCandidate(start string, src, dst *models.Position, min_radius, max_radius float32) ([]*models.JourneyLeg, error) {
	if db == nil || db.DB == nil {
		return nil, fmt.Errorf("Not connected to cache")
	}
	rows, err := db.Query(`
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

func NearestHub(owned bool, star string) (string, float32, error) {
	if db == nil || db.DB == nil {
		return "", 0, fmt.Errorf("Not connected to cache")
	}
	s, err := models.NewStar(star)
	if err != nil {
		return "", 0, err
	}
	fn := db.FindNearestHub
	if owned {
		fn = db.FindNearestOwnedHub
	}
	nearest, dist, err := fn(s.Position.X, s.Position.Y, s.Position.Z)
	if err != nil {
		return "", 0, fmt.Errorf("Can't find nearest hub: %v", err)
	}
	return nearest, dist, nil
}

func GetPartialJourney(j *models.Journey) (*models.Journey, error) {
	if db == nil || db.DB == nil {
		return j, fmt.Errorf("Not connected to cache")
	}
	src := j.Source
	dst := j.Dest
	row := db.QueryRow(`
			SELECT cached_journey_steps.journey_id FROM cached_journey_steps
			JOIN cached_journey ON cached_journey.id = cached_journey_steps.journey_id
			WHERE (cached_journey_steps.src = $1 OR cached_journey_steps.dest = $1)
			  AND cached_journey.max_hop <= $3
			  AND ($4 OR cached_journey.use_station = false)
			  AND ($5 OR cached_journey.use_hub = false)
			INTERSECT
			SELECT cached_journey_steps.journey_id FROM cached_journey_steps
			JOIN cached_journey ON cached_journey.id = cached_journey_steps.journey_id
			WHERE (cached_journey_steps.src = $2 OR cached_journey_steps.dest = $2)
			  AND cached_journey.max_hop <= $3
			  AND ($4 OR cached_journey.use_station = false)
			  AND ($5 OR cached_journey.use_hub = false)`, src, dst, j.MaxHop, j.UseStation, j.UseHub)
	var jid int
	if err := row.Scan(&jid); err != nil {
		Log("Can't find a partial journey (%s-%s): %v", src, dst, err)
		return j, nil
	}
	Log("Found partial route from %s to %s in JID %d", src, dst, jid)
	rows, err := db.Query(`
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

func IdentifyNetworkBridge(loc models.LocationID) (map[string]float32, error) {
	// get all the systems in range
	// check each for relays
	// group by the "oldest" member of the network
	// return the nearest star for each network
	star, err := models.NewStar(loc.Star())
	if err != nil {
		return nil, err
	}
	pos := star.Position
	near, err := db.QueryStarsInRadius(pos.X, pos.Y, pos.Z, 15, 0)
	if err != nil {
		return nil, err
	}
	devNet := make(map[string]string)
	getNID := func(ca *models.CodeAlias) (string, error) {
		if n, ok := devNet[ca.Alias()]; ok {
			return n, nil
		}
		net, err := rest.DeviceNetwork(ca)
		if err != nil {
			return "", fmt.Errorf("Can't get network for %q: %v", ca.Alias(), err)
		}
		var id *models.CodeAlias
		var nid string
		for _, c := range net.Connections {
			if id == nil || (c.DeviceCode.Type() == id.Type() && c.DeviceCode.Num() < id.Num()) || (c.DeviceCode.Type() == "fr" && id.Type() != "fr") {
				id = c.DeviceCode
				nid = fmt.Sprintf("%s (%s)", c.Star, id.Alias())
			}
		}
		for _, c := range net.Connections {
			devNet[c.DeviceCode.Alias()] = nid
		}
		return nid, nil
	}
	nets := make(map[string][]string)
	for _, s := range near {
		if s.Designation == loc.Star() {
			continue
		}
		// Log("Checking relays at %q (%.2f ly)", s.Designation, s.Distance)
		frs, err := db.QueryDevices(
			cache.QueryDevicesStatus("relaying"),
			cache.QueryDevicesLocation(s.Designation),
		)
		if err != nil {
			return nil, err
		}
		for _, fr := range frs {
			ca := models.NewCodeAlias(fr.Code)
			nid, err := getNID(ca)
			if err != nil {
				return nil, err
			}
			// Log("... %s: %s", ca, nid)
			if l, ok := nets[nid]; !ok || !slices.Contains(l, s.Designation) {
				nets[nid] = append(nets[nid], s.Designation)
			}
		}
	}
	res := make(map[string]float32)
	for _, s := range near {
		if s.Designation == loc.Star() {
			continue
		}
		net := s.Designation
		for nid, stars := range nets {
			if slices.Contains(stars, s.Designation) {
				net = nid
				break
			}
		}
		if _, ok := res[net]; !ok {
			res[net] = s.Distance
			continue
		}
		if s.Distance < res[net] {
			res[net] = s.Distance
		}
	}
	return res, nil
}

func NearestRelay(dest string, ignore *models.CodeAlias) (string, error) {
	// Get the home relay network
	net, err := FullNetwork(ignore)
	if err != nil {
		return "", err
	}
	star, err := models.NewStar(dest)
	if err != nil {
		return "", err
	}
	var closest float32
	var relay string
	for _, r := range net {
		rStar, err := models.NewStar(r)
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
