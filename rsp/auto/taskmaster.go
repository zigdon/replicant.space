package auto

import (
	"errors"
	"fmt"
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
//    priority: int, higher is more important
//    rate: int, how many devices to handle at a time, 0=unlimited
//  }

// task interface - specific tasks will implement these
type tmmTask interface {
	ID() int
	Type() string
	Title() string
	Desc() string
	Configure(map[string]any) error

	Ready() bool
	Process() error
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
		log("Processing task %q: %s", t.Title, t.Desc())
		// Check completion
		if t.Ready() {
			log("...Ready")
			errs = append(errs, t.Finish())
			continue
		}

		// Make progress
		errs = append(errs, t.Process())

		next = append(next, t)
	}
	tmm.tasks = next
	return errors.Join(errs...)
}

func (tmm *TaskMasterMachine) Name() string {
	return "Greg"
}

func (tmm *TaskMasterMachine) Process() (time.Time, error)  { return time.Time{}, nil }
func (tmm *TaskMasterMachine) SaveState(state string) error { return nil }
func (tmm *TaskMasterMachine) Status() string               { return "" }

func (tmm *TaskMasterMachine) loadTasks() error {
	rows, err := DB.Query(`
	SELECT id, added, started, type, title, detail
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
		var added, started int64
		var kind, title string
		var detail cache.JSONB[map[string]any]
		if err := rows.Scan(&id, &added, &started, &kind, &title, &detail); err != nil {
			return err
		}
		switch kind {
		case "stage":
			t := &tmmStage{
				id:     id,
				title:  title,
				dryRun: tmm.dryRun,
			}
			t.added = time.Unix(added, 0)
			t.started = time.Unix(started, 0)
			if err := t.Configure(detail.Data); err != nil {
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
		ts.priority = p.(int)
	} else {
		return fmt.Errorf("Missing priority: %v", conf)
	}
	if r, ok := conf["rate"]; ok {
		ts.rate = r.(int)
	} else {
		return fmt.Errorf("Missing rate: %v", conf)
	}
	if c, ok := conf["composition"]; ok {
		m, ok := c.(map[string]int)
		if !ok {
			return fmt.Errorf("Invalid composition: %v", c)
		}
		ts.composition = m
	} else {
		return fmt.Errorf("Missing composition: %v", conf)
	}
	return nil
}

func (ts *tmmStage) Ready() bool {
	// Check if all the desired devices have been delivered
	devs, err := rest.Devices(map[string]string{
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

func (ts *tmmStage) Process() error {
	devs, err := rest.Devices(map[string]string{
		"tag": fmt.Sprintf("task:%d", ts.id),
	})
	if err != nil {
		return fmt.Errorf("Error getting device list: %v", err)
	}

	var errs []error
	inProgress := make(map[string][]*models.CodeAlias)
	found := make(map[string]int)
	loaded := make(map[string]int)
	toLoad := make(map[string][]*models.CodeAlias)
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
			log("%s (%s) waiting at %s", d.Code, d.Type, d.Location)
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
		toLoad[string(d.Location)] = append(toLoad[string(d.Location)], d.Code)
	}

	// Find or build missing
	missing := make(map[string]int)
	for k, v := range ts.composition {
		if delta := v - found[k]; delta > 0 {
			log("Still missing %d %s", delta, k)
			missing[k] = delta
		}
	}
	/*
		if len(missing) > 0 {
		  for k, v := range missing {
			// Find untagged devices within range
			// Tag them for transport
		  }
		}
		if len(missing) > 0 {
		  for k, v := range missing {
			// Build mising devices
		  }
		}
	*/

	// Find or build transports
	// Ship
	return nil
}

func (ts *tmmStage) ID() int       { return ts.id }
func (ts *tmmStage) Type() string  { return "Stage" }
func (ts *tmmStage) Title() string { return ts.title }
func (ts *tmmStage) Desc() string  { return "TODO" }
func (ts *tmmStage) Start() error  { return nil }
func (ts *tmmStage) Finish() error { return nil }
