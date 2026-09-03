package auto

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/constants"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// - Read the current intent table
// - Read the current inventory table
// - Identify where inventory < intent
// - Find the nearest location that has the missing inventory
//   - But does not have an intent
// - Find CFs near the pickup locations, send them there
//   - Place a lien on the supply and demand so we don't double fill
// - On landing, stow/attach as needed, ship to destination
// - On landing, unload/detach as needed, update table

const distLimit = 200

type totals struct {
	collected float32
	delivered float32
}

type pickupTask struct {
	pickup    models.LocationID
	dropoff   models.LocationID
	ship      *models.Device
	resources map[string]int
	complete  bool
}

func (pt *pickupTask) String() string {
	var ship string
	if pt.ship != nil {
		ship = pt.ship.Code.Alias()
	}
	return fmt.Sprintf("Task: %s->%s %v %s", pt.pickup, pt.dropoff, pt.resources, ship)
}

func (pt *pickupTask) Empty() bool {
	for _, v := range pt.resources {
		if v > 0 {
			return false
		}
	}
	return true
}

type DispatchMachine struct {
	dryRun bool
	// location -> type -> qty
	supply map[string]map[string]int
	demand map[string]map[string]int
	// ship -> type -> qty
	manifest map[string]map[string]int

	// planned pickup tasks
	tasks []*pickupTask

	// print timer - only trigger prints once every 30m
	lastPrint time.Time

	// ftl network
	inRange map[string]bool

	// totals of a single pass
	stats map[string]*totals

	convoy *common.TravelCoordinator
}

func (dm *DispatchMachine) Start(dev *models.Device, dryRun bool) error {
	// Initialize vars
	dm.supply = make(map[string]map[string]int)
	dm.demand = make(map[string]map[string]int)
	dm.manifest = make(map[string]map[string]int)
	dm.inRange = make(map[string]bool)
	dm.dryRun = dryRun
	dm.lastPrint = time.Now()

	// Find our AFC
	afc := getTags(dev)["convoy"]
	if afc == "" {
		return fmt.Errorf("No AFC defined, add convoy:code tag to %s", dev.Code.Alias())
	}
	dm.convoy = common.NewTravelCoordinator(models.NewCodeAlias(afc), dryRun)
	return dm.UpdateState()
}

func (dm *DispatchMachine) UpdateState() error {
	// Refresh the current FTL network
	net, err := common.FullNetwork()
	if err != nil {
		return fmt.Errorf("Error getting ftl network: %v", err)
	}
	clear(dm.inRange)
	// The device location is not listed as a connection, add it manually
	dm.inRange["MENKUNT"] = true
	for _, c := range net {
		dm.inRange[c] = true
	}

	// Clear the current intent and inventory
	clear(dm.demand)
	clear(dm.supply)
	// Load the current intent
	rows, err := DB.Query(`
	    SELECT location, demand
		FROM intent
	`)
	if err != nil {
		return fmt.Errorf("Can't get intent: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var loc string
		var dem []byte
		if err := rows.Scan(&loc, &dem); err != nil {
			return fmt.Errorf("Can't scan intent: %v", err)
		}
		var demand map[string]int
		if err := json.Unmarshal(dem, &demand); err != nil {
			return fmt.Errorf("Can't parse intent: %v", err)
		}
		dm.demand[loc] = demand
	}
	// Load the updated inventory
	rows, err = DB.Query(`
	    SELECT designation, carbon, conductive, rares, silicates, structural, volatiles
		FROM inventory
	`)
	if err != nil {
		return fmt.Errorf("Can't get inventory: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var loc string
		var ca, co, ra, si, st, vo int
		if err := rows.Scan(&loc, &ca, &co, &ra, &si, &st, &vo); err != nil {
			return err
		}
		supply := map[string]int{
			"carbon":     ca,
			"conductive": co,
			"rares":      ra,
			"silicates":  si,
			"structural": st,
			"volatiles":  vo,
		}
		dm.supply[loc] = supply
	}
	// Load deliveris in flight
	rows, err = DB.Query(`
	    SELECT origin, destination, ship, cargo, data
		FROM deliveries JOIN json_devices on ship = code
	`)
	if err != nil {
		return fmt.Errorf("Can't get deliveries: %v", err)
	}
	defer rows.Close()
	dm.tasks = dm.tasks[:0]
	for rows.Next() {
		var from, to, ship string
		var cData, data []byte
		if err := rows.Scan(&from, &to, &ship, &cData, &data); err != nil {
			return err
		}
		var cargo map[string]int
		if err := json.Unmarshal(cData, &cargo); err != nil {
			return err
		}
		var device *models.Device
		if err := json.Unmarshal(data, &device); err != nil {
			return err
		}
		dm.manifest[ship] = cargo
		dm.tasks = append(dm.tasks, &pickupTask{
			pickup:    models.LocationID(from),
			dropoff:   models.LocationID(to),
			ship:      device,
			resources: cargo,
		})
	}

	return nil
}

func (dm *DispatchMachine) getSent(loc string) map[string]int {
	res := make(map[string]int)
	for _, t := range dm.tasks {
		if string(t.dropoff) != loc {
			continue
		}
		for k, v := range t.resources {
			res[k] += v
		}
	}
	return res
}

func (dm *DispatchMachine) balanceBooks() map[string]map[string]int {
	// Compare the intent (and in-progress) to reality
	toDeliver := make(map[string]map[string]int)

	var data [][]any
	demand := make(map[string]map[string]int)
	maps.Copy(demand, dm.demand)
	upkeep, err := common.GetUpkeep()
	if err != nil {
		log("Error getting upkeep: %v", err)
	} else {
		for loc, vs := range upkeep {
			if _, ok := demand[loc]; !ok {
				demand[loc] = vs
				continue
			}
			for k, v := range vs {
				demand[loc][k] += v
			}
		}
	}

	for loc, vs := range demand {
		sent := dm.getSent(loc)
		inv, ok := dm.supply[loc]
		if !ok {
			dm.supply[loc] = make(map[string]int)
		}
		if len(toDeliver[loc]) > 0 {
			data = append(data, []any{
				loc, vs, inv, sent, toDeliver[loc],
			})
		}
		for res, qty := range vs {
			if qty-sent[res]-inv[res] <= 0 {
				continue
			}
			if _, ok := toDeliver[loc]; !ok {
				toDeliver[loc] = make(map[string]int)
			}
			toDeliver[loc][res] += qty - sent[res] - inv[res]
		}
	}
	slices.SortFunc(data, func(a, b []any) int {
		return cmp.Compare(a[0].(string), b[0].(string))
	})
	common.PrintTable([]string{"Location", "Intent", "Inventory", "Incoming", "Missing"}, data)
	return toDeliver
}

func (dm *DispatchMachine) findSys(loc models.LocationID, missing map[string]int) ([]*pickupTask, error) {
	if !dm.inRange[loc.Star()] {
		return nil, fmt.Errorf("%s is not in FTL network", loc)
	}
	var tasks []*pickupTask
	// find nearby stars that have the required materials
	var fields []string
	var wheres []string
	var n = 1
	var total int
	var params []any
	for k, v := range missing {
		if v <= 0 {
			continue
		}
		fields = append(fields, k)
		total += v
		if v > 500 {
			wheres = append(wheres, fmt.Sprintf("%s >= 500", k))
		} else {
			wheres = append(wheres, fmt.Sprintf("%s >= $%d", k, n))
			params = append(params, v)
			n++
		}
	}
	if total == 0 {
		return tasks, fmt.Errorf("Nothing is missing at %s", loc)
	}

	avoid := append([]string{string(loc)}, constants.Homes...)
	params = append(params, pq.Array(avoid), loc.Star())
	q := fmt.Sprintf(`
		SELECT i.designation, %s
		FROM inventory i JOIN stars s ON i.star = s.designation
		WHERE i.designation != ALL($%d::TEXT[]) AND (%s)
		ORDER BY s.position <=> (
		  SELECT position
		  FROM stars
		  WHERE designation = $%d
		)`, strings.Join(fields, ", "), n, strings.Join(wheres, " OR "), n+1)
	rows, err := DB.Query(q, params...)
	if err != nil {
		return tasks, fmt.Errorf("Error finding potential systems: %v ", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log("Error closing query: %v", err)
		}
	}()

	for rows.Next() {
		// Figure out how many resources in the system are accounted for
		var qres []any
		var sys string
		qres = append(qres, &sys)
		for range fields {
			qres = append(qres, new(int))
		}
		if err := rows.Scan(qres...); err != nil {
			return tasks, fmt.Errorf("Error scanning systems: %v", err)
		}
		if !dm.inRange[models.LocationID(sys).Star()] {
			continue
		}
		good := false
		res := make(map[string]int)
		pending := dm.pendingPickup(sys)
		for n, f := range fields {
			i, ok := qres[n+1].(*int)
			if !ok {
				return tasks, fmt.Errorf("Expected %s to be an *int, got %v (%T)", f, qres[n+1], qres[n+1])
			}
			res[f] = *i - pending[f]
			if res[f] <= 0 {
				delete(res, f)
				continue
			}
			if res[f] >= min(missing[f], 500) {
				good = true
			}
		}
		if !good {
			continue
		}

		// Now create tasks for picking up as much as we need, or until the system runs out
		for {
			task := &pickupTask{
				pickup:    models.LocationID(sys),
				dropoff:   loc,
				resources: make(map[string]int),
			}
			space := 500
			for k, v := range missing {
				v = min(v, space, res[k])
				if v <= 0 {
					continue
				}
				task.resources[k] = v
				missing[k] -= v
				space -= v
				res[k] -= v
				if missing[k] <= 0 {
					delete(missing, k)
				}
				if res[k] <= 0 {
					delete(res, k)
				}
			}
			if task.Empty() {
				break
			}
			tasks = append(tasks, task)
		}
		if len(missing) == 0 {
			return tasks, nil
		}
	}
	return tasks, fmt.Errorf("%s: Could not find %v", loc, missing)
}

func (dm *DispatchMachine) releaseLostCargo() error {
	// Find "lost" freighters -- ones that are idle, with cargo, and no known purpose.
	rows, err := DB.Query(`
		SELECT code, location
		FROM json_devices LEFT JOIN deliveries ON code = ship
		WHERE id IS NULL
		  AND type = 'cargo_freighter'
		  AND status = 'idle'
		  AND data->'tags' = '[]'
		  AND data->>'cargo' IS NOT NULL
		  AND data->'cargo' != '[]'
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	lost := make(map[string][]string)
	for rows.Next() {
		var c, l string
		if err := rows.Scan(&c, &l); err != nil {
			return err
		}
		ca := models.NewCodeAlias(c)
		if res, err := deviceCommand(ca, "deposit_resources", nil, dm.dryRun); err != nil {
			log("Error droping inventory from %s: %v", ca.Alias(), err)
		} else {
			if dm.stats[l] == nil {
				dm.stats[l] = new(totals)
			}
			for _, v := range res.Deposited {
				dm.stats[l].delivered += v
			}
		}
		lost[l] = append(lost[l], ca.Alias())
	}
	if len(lost) > 0 {
		var data [][]any
		for loc, cas := range lost {
			slices.Sort(cas)
			data = append(data, []any{loc, strings.Join(cas, ", ")})
		}
		slices.SortFunc(data, func(a, b []any) int {
			return cmp.Compare(a[0].(string), b[0].(string))
		})
		log("Emptied %d lost ships:", len(data))
		common.PrintTable([]string{"Ship", "Location"}, data)
	}
	return nil
}

func (dm *DispatchMachine) Process() (time.Time, error) {
	dm.stats = make(map[string]*totals)
	eta := time.Now()
	if err := dm.UpdateState(); err != nil {
		return eta, err
	}
	if err := dm.releaseLostCargo(); err != nil {
		return eta, err
	}

	toDeliver := dm.balanceBooks()
	var errs []error
	var newTasks []*pickupTask
	for loc, missing := range toDeliver {
		tasks, err := dm.findSys(models.LocationID(loc), missing)
		if err != nil {
			errs = append(errs, err)
		} else {
			newTasks = append(newTasks, tasks...)
		}
	}

	if len(newTasks) > 0 {
		log("%d new pickup tasks:", len(newTasks))
		for _, t := range newTasks {
			log("  %s->%s: %v", t.pickup, t.dropoff, t.resources)
		}
		dm.tasks = append(dm.tasks, newTasks...)
	}

	// Handle all the pending deliveries:
	// - If unassigned, find a freighter near the pickup location, and send it there.
	// - If the assigned freighter is at the pickup, load up the resource, head to destination
	// - If the assigned freighter is at the destination, unload and remove the task
	// - If it's anywhere else, just delete the task, let a new one be minted
	log("%d tasks pending", len(dm.tasks))
	noShips := make(map[string]int)
	for n, t := range dm.tasks {
		log("[%d/%d] Processing delivery of %#v %s->%s",
			n, len(dm.tasks), t.resources, t.pickup, t.dropoff)
		if t.ship == nil {
			if noShips[string(t.pickup)] > 0 {
				continue
			}
			log("... finding a ship near %s", t.pickup)
			ship, err := dm.getShip(t.pickup)
			if err != nil {
				noShips[string(t.pickup)]++
				errs = append(errs, err)
				continue
			}
			dm.manifest[ship.Code.String()] = t.resources
			shipEta, err := dm.convoy.Queue(ship.Code, string(ship.Location), string(t.pickup))
			if err != nil {
				errs = append(errs, err)
				continue
			}
			eta = sooner(eta, shipEta)
			errs = append(errs,
				DB.AddDelivery(
					dm.dryRun, string(t.pickup), string(t.dropoff), ship.Code.String(), t.resources))
			t.ship = ship
			if !shipEta.IsZero() {
				continue
			}
		} else {
			info, err := rest.DeviceInfo(t.ship.Code)
			if err != nil {
				log("Error updating ship info %s: %v", t.ship.Code, err)
			} else {
				t.ship = info
			}
		}
		if t.Empty() {
			log("Empty task, skipping")
			t.complete = true
			continue
		}
		switch t.ship.Location {
		case "":
			tripEta := t.ship.Travel.Arrives.Time().Truncate(time.Second)
			log("%s is in transit to %s: %s (%s)",
				t.ship, t.ship.Travel.Destination, tripEta, time.Until(tripEta).Truncate(time.Second))
			eta = sooner(eta, tripEta)
		case t.dropoff:
			if len(t.ship.Cargo) > 0 {
				log("%s ready for drop-off at %s", t.ship, t.ship.Location)
				res, err := deviceCommand(t.ship.Code, "deposit_resources", nil, dm.dryRun)
				if err != nil {
					errs = append(errs,
						fmt.Errorf("%s can't deposit at %s: %v", t.ship.Code, t.pickup, err))
					continue
				}
				if dm.stats[string(t.dropoff)] == nil {
					dm.stats[string(t.dropoff)] = new(totals)
				}
				for _, v := range res.Deposited {
					dm.stats[string(t.dropoff)].delivered += v
				}
			}
			t.complete = true
		case t.pickup:
			log("%s ready for pick up at %s->%s", t.ship, t.ship.Location, t.dropoff)
			res := t.resources
			if len(t.ship.Cargo) > 0 {
				log("%s already has cargo:", t.ship)
				for _, c := range t.ship.Cargo {
					log("... %s", c.String())
					res[c.ResourceType] -= c.Quantity
					if res[c.ResourceType] <= 0 {
						delete(res, c.ResourceType)
					}
				}
			}
			if len(res) > 0 {
				res, err := deviceCommand(t.ship.Code, "collect_resources", map[string]any{
					"resources": res,
				}, dm.dryRun)
				if err != nil {
					errs = append(errs,
						fmt.Errorf("%s can't collect %v at %s: %v",
							t.ship.Code.Alias(), t.resources, t.pickup, err))
					// Refresh the inventory, reset the delivery task
					rest.Location(string(t.pickup))
					t.complete = true
					continue
				}
				if dm.stats[string(t.ship.Location)] == nil {
					dm.stats[string(t.ship.Location)] = new(totals)
				}
				for _, v := range res.Collected {
					dm.stats[string(t.ship.Location)].collected += v
				}
			}
			shipEta, err := dm.convoy.Queue(t.ship.Code, string(t.ship.Location), string(t.dropoff))
			if err != nil {
				errs = append(errs, err)
				continue
			}
			eta = sooner(eta, shipEta)
		default:
			if len(t.ship.Cargo) > 0 {
				log("%s @ %s off course, dropping cargo: %#v", t.ship.Code, t.ship.Location, t)
				_, err := deviceCommand(t.ship.Code, "deposit_resources", nil, dm.dryRun)
				if err != nil {
					errs = append(errs,
						fmt.Errorf("%s can't deposit at %s: %v", t.ship.Code, t.ship.Location, err))
					continue
				}
			}
			t.complete = true
		}
	}

	errs = append(errs, dm.convoy.Ship())

	var next []*pickupTask
	for _, t := range dm.tasks {
		if t.complete {
			log("Marking task complete: %v", t)
			delete(dm.manifest, t.ship.Code.String())
			if t.ship != nil {
				errs = append(errs, DB.ClearDelivery(dm.dryRun, t.ship.Code.String()))
				res, err := deviceCommand(t.ship.Code, "deposit_resources", nil, dm.dryRun)
				if err != nil {
					errs = append(errs, err)
				} else {
					if dm.stats[string(t.ship.Location)] == nil {
						dm.stats[string(t.ship.Location)] = new(totals)
					}
					for _, v := range res.Deposited {
						dm.stats[string(t.ship.Location)].delivered += v
					}
				}
			}
		} else {
			next = append(next, t)
		}
	}
	dm.tasks = next

	// If we're really missing a bunch of capacity in an area, queue up some
	capGap := make(map[string]int)
	for k, v := range noShips {
		base := common.ClosestHomes(models.LocationID(k))[0]
		capGap[base] += v
	}
	nextPrint := dm.lastPrint.Add(30 * time.Minute)
	log("Missing capacity, next print: %s (%s)", nextPrint, time.Until(nextPrint))
	for k, v := range capGap {
		log("  %s: %d", k, v)
	}

	if time.Now().After(nextPrint) {
		for k, v := range capGap {
			if v < 10 {
				continue
			}
			common.Print(k, "cargo_freighter", int(v/10), true, dm.dryRun, nil)
			dm.lastPrint = time.Now()
		}
	}

	var data [][]any
	for k, t := range dm.stats {
		data = append(data, []any{k, t.collected, t.delivered})
	}
	slices.SortFunc(data, func(a, b []any) int {
		return cmp.Compare(a[0].(string), b[0].(string))
	})
	common.PrintTable([]string{"Location", "Pickup", "Dropoff"}, data)

	return eta, errors.Join(errs...)
}

func (dm *DispatchMachine) SaveState(state string) error {
	return nil
}

func (dm *DispatchMachine) Status() string {
	return fmt.Sprintf("%d deliveries in flight", len(dm.tasks))
}

func (dm *DispatchMachine) Name() string {
	return "Dispatch machine"
}

func (dm *DispatchMachine) getShip(loc models.LocationID) (*models.Device, error) {
	// Get idle and empty ships by distance, filter out the ones already assigned
	rows, err := DB.Query(`
	  SELECT code, location, position<->(SELECT position FROM stars WHERE designation=$1) AS dist, data
	  FROM json_devices JOIN stars ON SPLIT_PART(location, '-', 1) = designation
	  WHERE type = 'cargo_freighter'
	    AND status = 'idle'
		AND (data->'cargo' = '[]'::jsonb OR data->>'cargo' IS NULL)
	  ORDER BY dist
  `, loc.Star())
	if err != nil {
		return nil, fmt.Errorf("Error querying for ships: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log("Error closing query: %v", err)
		}
	}()
	stats := make(map[string]int)
	for rows.Next() {
		var code, location string
		var dist float32
		var dev cache.JSONB[*models.Device]
		if err := rows.Scan(&code, &location, &dist, &dev); err != nil {
			return nil, fmt.Errorf("Error scanning: %v", err)
		}
		if _, ok := dm.manifest[code]; ok {
			stats["in manifest"]++
			continue
		}
		ca := models.NewCodeAlias(code)
		if dist > distLimit {
			stats["too far"]++
			continue
		}
		log("Found %s @ %s, %.2f LY away", ca.Alias(), location, dist)
		return dev.Data, nil
	}
	return nil, fmt.Errorf("No empty freighters found within %d LY of %q: %v", distLimit, loc, stats)
}

func (dm *DispatchMachine) pendingPickup(loc string) map[string]int {
	res := make(map[string]int)
	for _, t := range dm.tasks {
		if string(t.pickup) != loc {
			continue
		}
		for k, v := range t.resources {
			res[k] += v
		}
	}

	return res
}
