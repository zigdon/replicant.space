package common

import (
	"fmt"
	"io"
	"os"
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
	date := time.Now().Format(time.Stamp) + " - "
	if !strings.HasSuffix(tmpl, "\n") {
		tmpl += "\n"
	}
	fmt.Fprintf(LogFh, date+tmpl, args...)
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
