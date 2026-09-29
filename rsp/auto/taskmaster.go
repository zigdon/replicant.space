package auto

import (
	"errors"
	"fmt"
	"time"

	"github.com/zigdon/rsp/auto/tasks"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
)

// Process one-off tasks
// Common:
//    priority: int, lower sooner
//    dependencies: task IDs that must be complete before this task can be processed
//    notification: notification text once complete
// Type = stage:
//  {
//    location: desired destination
//    composition: list of device/qty
//  }
// Type = relocate:
// {
//    devices: list of device codes
//    tags: list of tags (i.e. all the devices that have ALL the tags set)
//    destination: delivery address
// }
// Type = print:
// {
//    type: device type
//    qty: how many to print
//    location: print location
//    reuse: if idle devices exist at the location, should they be used
//    tags: list of tags to add to the devices
// }
// Type = queue:
// {
//    trigger: idle, empty, detached
//    duration: how long must the trigger be true for before the action is taken
//    action: device command
//    args: map[string]any - args to the command
// }
//

type TaskMasterMachine struct {
	dryRun bool
	tasks  []tasks.Task

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
	var next []tasks.Task
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
		SELECT id, added, started, type, title, details, state
		FROM tasks
		WHERE completed IS NULL
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var allTasks []tasks.Task
	for rows.Next() {
		var id int
		var added, started time.Time
		var kind, title string
		var details cache.JSONB[map[string]any]
		var state []byte
		if err := rows.Scan(&id, &added, &started, &kind, &title, &details, &state); err != nil {
			return err
		}
		switch kind {
		case "stage":
			t, err := newTask[*tasks.Stage](id, title, tmm.dryRun, added, started, details.Data, state)
			if err != nil {
				return err
			}

			allTasks = append(allTasks, t)
		default:
			return fmt.Errorf("Unknown task type %q for ID %d", kind, id)
		}
	}
	tmm.tasks = allTasks

	return nil
}

func newTask[T tasks.Task](id int, title string, dryRun bool, added, started time.Time, cfg map[string]any, state []byte) (tasks.Task, error) {
	cfg["id"] = id
	cfg["title"] = title
	cfg["dryRun"] = dryRun
	cfg["added"] = added
	cfg["started"] = started
	return (*new(T)).New(DB, cfg, state)
}
