package auto

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zigdon/rsp/common"
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

type DispatchMachine struct {
	dryRun bool
	// location -> type -> qty
	supply map[string]map[string]int
	demand map[string]map[string]int
	// ship -> type -> qty
	manifest map[string]map[string]int

	// planned pickup tasks
	tasks []*pickupTask
}

func (dm *DispatchMachine) Start(_ *models.Device, dryRun bool) error {
	// Nothing much to do here.
	dm.supply = make(map[string]map[string]int)
	dm.demand = make(map[string]map[string]int)
	dm.manifest = make(map[string]map[string]int)
	dm.dryRun = dryRun
	return dm.UpdateState()
}
func (dm *DispatchMachine) UpdateState() error {
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
	if err := rows.Close(); err != nil {
		return err
	}
	// Load the updated inventory
	rows, err = DB.Query(`
	    SELECT designation, carbon, conductive, rares, silicates, structural, volatiles
		FROM inventory
	`)
	if err != nil {
		return fmt.Errorf("Can't get inventory: %v", err)
	}
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
	if err := rows.Close(); err != nil {
		return err
	}
	// Load deliveris in flight
	rows, err = DB.Query(`
	    SELECT origin, destination, ship, cargo, data
		FROM deliveries JOIN json_devices on ship = code
	`)
	if err != nil {
		return fmt.Errorf("Can't get deliveries: %v", err)
	}
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
	if err := rows.Close(); err != nil {
		return err
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
	for loc, vs := range dm.demand {
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
	var tasks []*pickupTask
	// find nearby stars that have the required materials
	var fields []string
	var wheres []string
	var n = 1
	var total int
	var params []any
	for k, v := range missing {
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

	params = append(params, loc.Star())
	rows, err := DB.Query(fmt.Sprintf(`
		SELECT i.designation, %s
		FROM inventory i JOIN stars s ON i.star = s.designation
		WHERE %s
		ORDER BY s.position <=> (
		  SELECT position
		  FROM stars
		  WHERE designation = $%d
		)`, strings.Join(fields, ", "), strings.Join(wheres, " OR "), n), params...)
	if err != nil {
		return tasks, fmt.Errorf("Error finding potential systems: %v ", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log("Error closing query: %v", err)
		}
	}()

	for rows.Next() {
		var qres []any
		var sys string
		qres = append(qres, &sys)
		for range fields {
			qres = append(qres, new(int))
		}
		if err := rows.Scan(qres...); err != nil {
			return tasks, fmt.Errorf("Error scanning systems: %v", err)
		}
		good := true
		res := make(map[string]int)
		pending := dm.pendingPickup(sys)
		for n, f := range fields {
			i, ok := qres[n+1].(*int)
			if !ok {
				return tasks, fmt.Errorf("Expected %s to be an *int, got %v (%T)", f, qres[n+1], qres[n+1])
			}
			res[f] = *i - pending[f]
			if res[f] <= missing[f] {
				good = false
				break
			}
		}
		if good {
			task := &pickupTask{
				pickup:    models.LocationID(sys),
				dropoff:   loc,
				resources: make(map[string]int),
			}
			space := 500
			for k, v := range missing {
				v = min(v, space, res[k])
				task.resources[k] = v
				missing[k] -= v
				space -= v
				if missing[k] <= 0 {
					delete(missing, k)
				}
			}
			tasks = append(tasks, task)
			if len(missing) == 0 {
				return tasks, nil
			}
		}
	}
	return tasks, fmt.Errorf("Could not find %v", missing)
}

func (dm *DispatchMachine) Process() (time.Time, error) {
	eta := time.Now()
	if err := dm.UpdateState(); err != nil {
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
		log("New pickup tasks:")
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
	for _, t := range dm.tasks {
		log("Processing delivery of %#v %s->%s", t.resources, t.pickup, t.dropoff)
		if t.ship != nil {
			info, err := rest.DeviceInfo(t.ship.Code)
			if err != nil {
				log("Error updating ship info %s: %v", t.ship.Code, err)
			} else {
				t.ship = info
			}
		}
		switch {
		case t.ship == nil:
			log("... finding a ship near %s", t.pickup)
			ship, err := dm.getShip(t.pickup)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			dm.manifest[ship.Code.String()] = t.resources
			shipEta, err := common.Travel(ship.Code, string(t.pickup), dm.dryRun)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			eta = sooner(eta, shipEta)
			errs = append(errs,
				DB.AddDelivery(
					dm.dryRun, string(t.pickup), string(t.dropoff), ship.Code.String(), t.resources))
			t.ship = ship
		case t.ship.Location == "":
			tripEta := t.ship.Travel.Arrives.Time().Truncate(time.Second)
			log("%s is in transit to %s: %s (%s)",
				t.ship, t.ship.Travel.Destination, tripEta, time.Until(tripEta).Truncate(time.Second))
			eta = sooner(eta, tripEta)
		case t.ship.Location == t.dropoff:
			if len(t.ship.Cargo) > 0 {
				log("%s ready for drop-off at %s", t.ship, t.ship.Location)
				_, err := deviceCommand(t.ship.Code, "deposit_resources", nil, dm.dryRun)
				if err != nil {
					errs = append(errs,
						fmt.Errorf("%s can't deposit at %s: %v", t.ship.Code, t.pickup, err))
					continue
				}
			}
			delete(dm.manifest, t.ship.Code.String())
			t.complete = true
		case t.ship.Location == t.pickup:
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
				_, err := deviceCommand(t.ship.Code, "collect_resources", map[string]any{
					"resources": res,
				}, dm.dryRun)
				if err != nil {
					errs = append(errs,
						fmt.Errorf("Can't %s can't collect %v at %s: %v",
							t.ship.Code, t.resources, t.pickup, err))
					// Refresh the inventory, reset the delivery task
					rest.Location(string(t.pickup))
					t.complete = true
					continue
				}
			}
			shipEta, err := common.Travel(t.ship.Code, string(t.dropoff), dm.dryRun)
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
			delete(dm.manifest, t.ship.Code.String())
			t.complete = true
		}
	}

	var next []*pickupTask
	for _, t := range dm.tasks {
		if t.complete {
			log("Marking task complete: %v", t)
			errs = append(errs, DB.ClearDelivery(dm.dryRun, t.ship.Code.String()))
		} else {
			next = append(next, t)
		}
	}
	dm.tasks = next

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
	  WHERE type = 'cargo_freighter' AND status = 'idle' AND data->'cargo' = '[]'::jsonb
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
	for rows.Next() {
		var code, location string
		var dist float32
		var data []byte
		if err := rows.Scan(&code, &location, &dist, &data); err != nil {
			return nil, fmt.Errorf("Error scanning: %v", err)
		}
		if _, ok := dm.manifest[code]; ok {
			continue
		}
		ca := models.NewCodeAlias(code)
		var dev *models.Device
		if err := json.Unmarshal(data, &dev); err != nil {
			return nil, fmt.Errorf("Failed to unmarshal data for %s: %v\n%s", ca, err, data)
		}
		log("Found %s @ %s, %.2f LY away", ca.Alias(), location, dist)
		return dev, nil
	}
	return nil, fmt.Errorf("No freighter found")
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
