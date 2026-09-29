package tasks

import (
	"fmt"
	"time"

	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// task interface - specific tasks will implement these
type Task interface {
	New(db *cache.Cache, config map[string]any, data []byte) (Task, error)
	ID() int
	Type() string
	Title() string
	Desc() string

	Ready() bool
	Blocked() bool
	Process() (time.Time, error)
	Start() error
	Finish() error
}

// Aliases to make life easier
func log(tmpl string, args ...any) {
	common.LogLevel(2, tmpl, args...)
}

func sooner(a, b time.Time) time.Time {
	return common.Sooner(a, b)
}

func later(a, b time.Time) time.Time {
	return common.Later(a, b)
}

// Helpers copied from auto/state
func deviceCommand(id *models.CodeAlias, cmd string, args map[string]any, dryRun bool) (*models.CommandResp, error) {
	if dryRun {
		log("[DRYRUN] Issuing %q to %s: %v", cmd, id.Alias(), args)
		return nil, nil
	}
	log("Issuing %q to %s: %v", cmd, id.Alias(), args)
	res, err := rest.DeviceCommand[models.CommandResp](id, cmd, args)
	if err != nil {
		return res, fmt.Errorf("Error sending %q command to %q: %v", cmd, id.Alias(), err)
	}
	return res, nil
}
