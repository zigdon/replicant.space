package auto

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// Process one-off tasks
// Type = stage:
//  {
//    location: desired destination
//    composition: list of device/qty
//    priority: int, (TODO) higher is more important
//    rate: int, (TODO) how many devices to handle at a time, 0=unlimited
//  }

// task interface - specific tasks will implement these
type tmmTask interface {
	ID() int
	Type() string
	Title() string
	Desc() string
	Configure(map[string]any) error

	Ready() bool
	Process() (time.Time, error)
	Start() error
	Finish() error
}

type TaskMasterMachine struct {
	dryRun bool
	tasks  []tmmTask

	convoy *common.TravelCoordinator
}

func (tmm *TaskMasterMachine) Start(_ *models.Device, dryRun bool) error {
	tmm.dryRun = dryRun
	if err := tmm.loadTasks(); err != nil {
		return err
	}

	return tmm.UpdateState()
}

func (tmm *TaskMasterMachine) UpdateState() error {
	var errs []error
	var next []tmmTask
	for _, t := range tmm.tasks {
		log("Processing task %q: %s", t.Title(), t.Desc())
		// Check completion
		if t.Ready() {
			log("...Ready")
			errs = append(errs, t.Finish())
			continue
		}
		next = append(next, t)
	}
	tmm.tasks = next
	return errors.Join(errs...)
}

func (tmm *TaskMasterMachine) Name() string {
	return "Greg"
}

func (tmm *TaskMasterMachine) Process() (time.Time, error) {
	var errs []error
	var eta time.Time
	for _, t := range tmm.tasks {
		log("Processing task %q: %s", t.Title(), t.Desc())

		// Make progress
		taskEta, err := t.Process()
		errs = append(errs, err)
		eta = sooner(eta, taskEta)
	}

	return eta, errors.Join(errs...)
}
func (tmm *TaskMasterMachine) SaveState(state string) error { return nil }
func (tmm *TaskMasterMachine) Status() string               { return "" }

func (tmm *TaskMasterMachine) loadTasks() error {
	rows, err := DB.Query(`
	SELECT id, added, started, type, title, details
	FROM tasks
	WHERE completed IS NULL
  `)
	if err != nil {
		return err
	}
	defer rows.Close()
	var tasks []tmmTask
	for rows.Next() {
		var id int
		var added, started time.Time
		var kind, title string
		var details cache.JSONB[map[string]any]
		if err := rows.Scan(&id, &added, &started, &kind, &title, &details); err != nil {
			return err
		}
		switch kind {
		case "stage":
			t := &tmmStage{
				id:     id,
				title:  title,
				dryRun: tmm.dryRun,
			}
			t.added = added
			t.started = started
			if err := t.Configure(details.Data); err != nil {
				return err
			}

			tasks = append(tasks, t)
		default:
			return fmt.Errorf("Unknown task type %q for ID %d", kind, id)
		}
	}
	tmm.tasks = tasks

	return nil
}

//////////////////////// Specific tasks

const stageSearchDist = 100

type tmmStage struct {
	id      int
	title   string
	added   time.Time
	started time.Time
	dryRun  bool

	location    models.LocationID
	composition map[string]int
	priority    int
	rate        int
}

func (ts *tmmStage) Configure(conf map[string]any) error {
	if l, ok := conf["location"]; ok {
		ts.location = models.LocationID(l.(string))
	} else {
		return fmt.Errorf("Missing location: %v", conf)
	}
	if p, ok := conf["priority"]; ok {
		ts.priority = int(p.(float64))
	} else {
		return fmt.Errorf("Missing priority: %v", conf)
	}
	if r, ok := conf["rate"]; ok {
		ts.rate = int(r.(float64))
	} else {
		return fmt.Errorf("Missing rate: %v", conf)
	}
	if c, ok := conf["composition"]; ok {
		m, ok := c.(map[string]any)
		if !ok {
			return fmt.Errorf("Invalid composition: %v (%T)", c, c)
		}
		ts.composition = make(map[string]int)
		for k, v := range m {
			ts.composition[k] = int(v.(float64))
		}
	} else {
		return fmt.Errorf("Missing composition: %v", conf)
	}
	return nil
}

func (ts *tmmStage) Ready() bool {
	// Check if all the desired devices have been delivered
	devs, err := rest.Devices(map[string]any{
		"location": string(ts.location),
		"tag":      fmt.Sprintf("task:%d", ts.id),
	})
	if err != nil {
		log("Error checking devices at %s: %v", ts.location, err)
		return false
	}
	found := make(map[string]int)
	for _, d := range devs {
		if d.AttachedToDeviceCode != nil {
			log("%s is still attached to %s", d.Code, d.AttachedToDeviceCode)
			continue
		}
		found[d.Type]++
	}
	for k, v := range ts.composition {
		if found[k] < v {
			log("Not ready: %s@%s: want %d, found %d", k, ts.location, v, found[k])
			return false
		}
	}
	return true
}

func (ts *tmmStage) Process() (time.Time, error) {
	tag := fmt.Sprintf("task:%d", ts.id)
	var eta time.Time
	log("Task tag: %q", tag)
	devs, err := rest.Devices(map[string]any{
		"tag": tag,
	})
	if err != nil {
		return eta, fmt.Errorf("Error getting device list: %v", err)
	}

	var errs []error
	inProgress := make(map[string][]*models.CodeAlias)
	found := make(map[string]int)
	loaded := make(map[string]int)
	pickup := make(map[string][]*models.CodeAlias)
	// Check for devices that were already identified and are in transit:
	// Found -> Loaded -> Transit -> Arrived -> Detached
	for _, d := range devs {
		inProgress[string(d.Location)] = append(inProgress[string(d.Location)], d.Code)
		found[d.Type]++
		if d.Location == ts.location {
			if d.AttachedToDeviceCode != nil {
				_, err := deviceCommand(d.AttachedToDeviceCode, "detach", map[string]any{"target": d.Code}, ts.dryRun)
				errs = append(errs, err)
			}
			log("%s (%s) ready at %s", d.Code, d.Type, d.Location)
			continue
		}
		if d.Location == "" {
			log("%s (%s) is in transit: ETA %s (%s)",
				d.Code, d.Type, d.Travel.Arrives, time.Until(d.Travel.Arrives.Time()))
			continue
		}
		if d.AttachedToDeviceCode != nil {
			log("%s (%s) is loaded on %s at %s", d.Code, d.Type, d.AttachedToDeviceCode, d.Location)
			loaded[d.AttachedToDeviceCode.Alias()]++
			continue
		}
		log("%s (%s) is waiting to be loaded at %s", d.Code, d.Type, d.Location)
		pickup[string(d.Location)] = append(pickup[string(d.Location)], d.Code)
	}

	// Find or build missing
	missing := make(map[string]int)
	for k, v := range ts.composition {
		if delta := v - found[k]; delta > 0 {
			log("Still missing %d %s", delta, k)
			missing[k] = delta
		}
	}
	if len(missing) > 0 {
		for k, v := range missing {
			// Find untagged devices within range
			// TODO: don't bother using a spare if it'll take longer to
			// ship it than to print a new one
			spares, err := rest.Devices(map[string]any{"device_type": k, "untagged": true})
			if err != nil {
				errs = append(errs, err)
				continue
			}
			for _, d := range spares {
				log("checking %s (%s)", d.Code, d.Type)
				if d.Location == "" {
					continue
				}
				if dist, err := common.Distance(
					d.Location.Star(), ts.location.Star()); err == nil && dist > stageSearchDist {
					log("%s is too far away from %s: %.2f ly", d.Code, ts.location, dist)
					continue
				} else if err != nil {
					log("Can't get distance to %s @ %s: %v", d.Code, d.Location, err)
					continue
				}
				// Tag them for transport
				if err := ts.setTag(d.Code); err != nil {
					log("Error tagging %s: %v", d.Code, err)
					continue
				}
				v--
				pickup[string(d.Location)] = append(pickup[string(d.Location)], d.Code)
				if v <= 0 {
					delete(missing, k)
					break
				}
			}
		}
	}

	if len(missing) > 0 {
		home := common.ClosestHomes(ts.location)[0]
		// Build mising devices
		log("Still missing %v", missing)
		for k, v := range missing {
			cfg := map[string]any{
				"tags": []string{tag},
			}
			if bp := common.GetBP(k); bp != nil && slices.Contains(bp.Features, "modular") {
				cfg["flatpack"] = true
			}
			plan, err := common.Print(home, k, v, true, ts.dryRun, cfg)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			for range v {
				pickup[home] = append(pickup[home], nil)
			}
			eta = sooner(eta, plan.ETA)
		}
	}

	// Find or build transports
	for loc, devs := range pickup {
		var txs []*models.Device
		need := len(devs)
		add := func(loc string, devType string) error {
			cfg := map[string]any{"device_type": devType}
			if loc != "" {
				cfg["location"] = loc
			}
			// First already tagged
			cfg["tag"] = tag
			ds, err := rest.Devices(cfg)
			if err != nil {
				return err
			}
			txs = append(txs, ds...)
			// Then untagged
			delete(cfg, "tag")
			cfg["untagged"] = true
			ds, err = rest.Devices(cfg)
			if err != nil {
				return err
			}
			txs = append(txs, ds...)
			return nil
		}
		// First, local fleets, always good
		errs = append(errs, add(loc, "mobile_fleet"))
		// Then, smaller platforms, if they're big enough
		if len(devs) <= 4 {
			errs = append(errs, add(loc, "surge_platform"))
		}
		if len(devs) == 1 {
			errs = append(errs, add(loc, "surge_plate"))
		}

		// If we found any local platforms, tag them
		for _, tx := range txs {
			// Untagged platforms must be empty
			if len(tx.AttachedDevices) > 0 && len(tx.Tags) == 0 {
				continue
			}
			// Full platforms are full
			if len(tx.AttachedDevices) == tx.AttachCapacity {
				continue
			}
			// Keep track of how many devices we can ship
			need -= tx.AttachCapacity - len(tx.AttachedDevices)
			// Add the tag
			errs = append(errs, ts.setTag(tx.Code))
		}

		// If we need more slots, see if we can find platforms nearby (100ly)
		if need > 0 {
			// Check for platforms already inbound
			inbound, err := rest.Devices(map[string]any{"location": "", "tag": tag})
			if err != nil {
				errs = append(errs, err)
				continue
			}
			for _, d := range inbound {
				if string(d.Travel.Destination) == loc {
					need -= d.AttachCapacity
				}
			}
			if need <= 0 {
				break
			}

			// Find nearby platforms
			txs = txs[:0]
			errs = append(errs, add("", "mobile_fleet"))
			if need <= 4 {
				errs = append(errs, add("", "surge_platform"))
			}
			if need == 1 {
				errs = append(errs, add("", "surge_plate"))
			}
			for _, d := range txs {
				if len(d.AttachedDevices) > 0 {
					continue
				}
				if dist, err := common.Distance(d.Location.Star(), loc); err == nil && dist > 100 {
					log("%s is too far away from %s: %.2f ly", d.Code, loc, dist)
				} else if err != nil {
					log("Can't get distance to %s @ %s: %v", d.Code, d.Location, err)
					continue
				}
				errs = append(errs, ts.setTag(d.Code))
				_, err := common.Travel(d.Code, loc, ts.dryRun)
				errs = append(errs, err)
				need -= d.AttachCapacity
				if need <= 0 {
					break
				}
			}
		}

		// If we still need more slots, print a transport
		if need > 0 {
			home := common.ClosestHomes(models.LocationID(loc))[0]
			txType := "surge_platform"
			if need > 4 {
				txType = "mobile_fleet"
			}
			plan, err := common.Print(home, txType, 1, true, ts.dryRun, map[string]any{"tags": []string{tag}})
			if err != nil {
				errs = append(errs, err)
			}
			eta = sooner(eta, plan.ETA)
		}
	}

	// Load and ship
	// Now that we have a set of platforms available, load what we can, ship what's ready
	txs, err := rest.Devices(map[string]any{"tag": tag})
	if err != nil {
		errs = append(errs, err)
	}
	var mopup []*models.Device
	for _, tx := range txs {
		if tx.Location == "" {
			// In transit, skip for now
			eta = sooner(eta, tx.Travel.Arrives.Time())
			continue
		}

		// If there are devices to pick up here, and there's room, do
		space := tx.AttachCapacity - len(tx.AttachedDevices)
		if space > 0 {
			pickCount := len(pickup[string(tx.Location)])
			pick := pickup[string(tx.Location)][:min(space, pickCount)]
			_, err := deviceCommand(tx.Code, "attach", map[string]any{"targets": pick}, ts.dryRun)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			pickup[string(tx.Location)] = slices.Delete(pickup[string(tx.Location)], 0, len(pick))
			space -= len(pick)
		}

		if len(pickup[string(tx.Location)]) == 0 {
			delete(pickup, string(tx.Location))
		}

		// Full platforms can ship
		if space == 0 {
			_, err := common.Travel(tx.Code, string(ts.location), ts.dryRun)
			errs = append(errs, err)
			continue
		}

		// If there's nothing left to pick up here, add to a mopup list
		mopup = append(mopup, tx)
	}

	// If there's nothing left to pick up, or nothing left to pick it up with, we're done.
	if len(mopup) == 0 || len(pickup) == 0 {
		return eta, errors.Join(errs...)
	}

	// Now that everything local was picked up, see what's left to collect
	for loc, devs := range pickup {
		// Find the nearest mopup ship
		dists := make(map[models.LocationID]float32)
		for _, m := range mopup {
			if _, ok := dists[m.Location]; ok {
				continue
			}
			d, err := common.Distance(loc, string(m.Location))
			if err != nil {
				errs = append(errs, err)
				d = 999999
			}
			dists[m.Location] = d
		}
		slices.SortFunc(mopup, func(a, b *models.Device) int {
			return cmp.Compare(dists[a.Location], dists[b.Location])
		})
		need := len(devs)
		for _, tx := range mopup {
			txEta, err := common.Travel(tx.Code, loc, ts.dryRun)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			eta = sooner(eta, txEta)
			need -= tx.AttachCapacity - len(tx.AttachedDevices)
			if need <= 0 {
				break
			}
		}
	}
	return eta, errors.Join(errs...)
}

func (ts *tmmStage) ID() int       { return ts.id }
func (ts *tmmStage) Type() string  { return "Stage" }
func (ts *tmmStage) Title() string { return ts.title }
func (ts *tmmStage) Desc() string  { return "TODO" }
func (ts *tmmStage) Start() error  { return nil }
func (ts *tmmStage) Finish() error { return nil }

func (ts *tmmStage) setTag(id *models.CodeAlias) error {
	tag := fmt.Sprintf("task:%d", ts.ID())
	if ts.dryRun {
		log("[DRYRUN] Would set tag %q on %s", tag, id)
		return nil
	}
	return rest.UpdateTags(id, rest.AddTag, []string{tag})
}

func (ts *tmmStage) clearTag(id *models.CodeAlias) error {
	tag := fmt.Sprintf("task:%d", ts.ID())
	if ts.dryRun {
		log("[DRYRUN] Would clear tag %q on %s", tag, id)
		return nil
	}
	return rest.UpdateTags(id, rest.DelTag, []string{tag})
}
