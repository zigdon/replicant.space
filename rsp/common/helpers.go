package common

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var db *cache.Cache
var bps map[string]*models.Blueprint
var LogFh io.Writer = os.Stderr

func Log(tmpl string, args ...any) {
	var getCaller func(int) string
	getCaller = func(skip int) string {
		_, file, line, ok := runtime.Caller(skip)
		if !ok {
			return ""
		}

		paths := strings.Split(file, "/")
		file = paths[len(paths)-1]
		if file == "output.go" {
			// +2 because we're adding another frame by recursing
			return getCaller(skip + 2)
		}
		return fmt.Sprintf("%s:%d", file, line)
	}
	for n, a := range args {
		if a == nil {
			args[n] = fmt.Sprintf("nil(T:%T)", a)
			continue
		}
		switch a := a.(type) {
		case time.Time:
			args[n] = a.Truncate(time.Second).Format(time.Stamp)
		case time.Duration:
			args[n] = a.Truncate(time.Second)
		case *models.CodeAlias:
			args[n] = a.Alias()
		case *models.JSONTime:
			args[n] = a.Time().Truncate(time.Second).Format(time.Stamp)
		case *models.JSONTimeDelta:
			args[n] = a.Duration().Truncate(time.Second).String()
		case *models.Device:
			args[n] = a.Code.Alias()
		case []*models.Device:
			var ids []string
			for _, d := range a {
				ids = append(ids, d.Code.Alias())
			}
			args[n] = ids
		}
	}
	prefix := time.Now().Format("01-02 15:04:05") + " - "
	if caller := getCaller(2); caller != "" {
		prefix += caller + " - "
	}
	if !strings.HasSuffix(tmpl, "\n") {
		tmpl += "\n"
	}
	fmt.Fprintf(LogFh, prefix+tmpl, args...)
}

func ConnectDB(cdb *cache.Cache) {
	db = cdb
}

func AliasType(in string) (string, string) {
	if db == nil {
		return "", ""
	}
	return db.GetAliasAndType(in)
}

func Alias(in string) string {
	if db == nil {
		return in
	}
	// Check if there's already an alias
	out := db.HasAlias(in)
	if out != "" {
		return out
	}

	// If it doesn't look like a code, don't try to look it up
	if strings.ToUpper(in) != in {
		return in
	}

	// No alias, get the device type before making one
	deviceType, err := rest.GetType(in)
	if err != nil || deviceType == "" {
		return in
	}
	out, err = db.Alias(in, deviceType)
	if err != nil {
		Log("Error creating alias for %q(%s): %v", in, deviceType, err)
	}
	return out
}

func Aliases(in []*models.CodeAlias) []string {
	res := make([]string, len(in))
	for i, ca := range in {
		res[i] = ca.Alias()
	}
	return res
}

func Unalias(in string) string {
	if db == nil {
		return in
	}
	return db.Dealias(in)
}

func IsResource(in string) bool {
	return slices.Contains([]string{
		"carbon",
		"conductive",
		"rares",
		"silicates",
		"structural",
		"volatiles",
	}, in)
}

func GetBP(bp string) *models.Blueprint {
	if bps == nil {
		bps = make(map[string]*models.Blueprint)
	}
	if b, ok := bps[bp]; ok {
		return b
	}
	b := &models.Blueprint{DeviceType: bp}
	if err := b.Get(); err != nil {
		panic(fmt.Sprintf("Can load blueprint for %s: %v", bp, err))
	}
	bps[bp] = b
	return b
}

func GetFilteredDevices(devTypes, locations, statuses []string) ([]*models.CodeAlias, error) {
	getDevsAt := func(location, devType string) ([]*models.Device, error) {
		filter := make(map[string]string)
		if location != "" {
			filter["location"] = location
		}
		if devType != "" {
			filter["device_type"] = devType
		}

		devs, err := rest.Devices(filter)
		if err != nil {
			return nil, err
		}
		Log("%v: %d found", filter, len(devs))
		return devs, nil
	}

	if len(locations) == 0 {
		locations = append(locations, "")
	}
	if len(devTypes) == 0 {
		devTypes = append(devTypes, "")
	}
	var errs []error
	var devs []*models.Device
	for _, l := range locations {
		for _, t := range devTypes {
			ds, err := getDevsAt(l, t)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			devs = append(devs, ds...)
		}
	}

	Log("Searching for %v devices at %v: %d found", devTypes, locations, len(devs))
	var ids []*models.CodeAlias
	for _, d := range devs {
		if len(statuses) > 0 && !slices.Contains(statuses, d.Status) {
			continue
		}
		ids = append(ids, d.Code)
	}
	return ids, nil
}

func filterEmpty[T any](s []T, keep []bool) []T {
	var res []T
	for i, c := range s {
		if !keep[i] {
			continue
		}
		res = append(res, c)
	}
	return res
}

func stringify(in any) string {
	var out string
	switch a := in.(type) {
	case string:
		out = a
	case int:
		out = d(a)
	case float32:
		out = f(a)
	case time.Time:
		out = T(a)
	case time.Duration:
		out = Dt(a)
	case *models.DevicePointer:
		out = a.Code.Alias()
	case *models.Device:
		out = a.Code.Alias()
	case *models.CodeAlias:
		out = a.Alias()
	case *models.Position:
		out = a.String()
	case *models.JSONTime:
		out = a.String()
	case *models.JSONTimeDelta:
		out = a.String()
	case models.LocationID:
		out = string(a)
	default:
		out = v(a)
	}
	return out
}

func CountList(in []string) string {
	m := make(map[string]int)
	for _, i := range in {
		m[i]++
	}
	var names []string
	for k := range m {
		names = append(names, k)
	}
	slices.Sort(names)
	var res []string
	for _, n := range names {
		res = append(res, fmt.Sprintf("%d x %s", m[n], n))
	}
	return strings.Join(res, ", ")
}

func GetPrintQueueETA(dev *models.Device) time.Duration {
	if dev.Printing == nil && len(dev.PrintQueue) == 0 {
		return 0
	}

	var res time.Duration
	if dev.Printing != nil {
		res += dev.Printing.Eta.Duration()
	}
	for _, q := range dev.PrintQueue {
		res += GetBP(q.Type).PrintTime.Duration()
	}

	return res
}
