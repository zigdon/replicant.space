package models

import (
	"fmt"
	"time"

	"encoding/json"

	"github.com/zigdon/rsp/cache"
)

var db *cache.Cache

type Fillable interface {
	Fill() error
}

func fillTime(ts string, dest *time.Time) error {
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return err
	}
	*dest = parsed
	return nil
}

func fillDuration(secs float32, dest *time.Duration) error {
	parsed, err := time.ParseDuration(fmt.Sprintf("%.2fs", secs))
	if err != nil {
		return err
	}
	*dest = parsed
	return nil
}

func Parse[T any](data []byte) (*T, error) {
	s := new(T)

	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("Error parsing %T: %v", s, err)
	}
	if f, ok := any(s).(Fillable); ok {
		if err := f.Fill(); err != nil {
			return s, err
		}
	}

	return s, nil
}

func ConnectDB(cdb *cache.Cache) {
	db = cdb
}

type CodeAlias struct {
	orig  string
	alias string
}

func (a *CodeAlias) UnmarshalJSON(data []byte) error {
	var code string
	if err := json.Unmarshal(data, &code); err != nil {
		return err
	}
	if db == nil {
		// No database, just return this unmodified.
		*a = CodeAlias{orig: code}
		return nil
	}

	alias, err := db.Alias(code, "")
	if err != nil {
		return err
	}
	*a = CodeAlias{orig: code, alias: alias}

	return nil
}

func (a *CodeAlias) String() string {
	if a != nil {
		return a.orig
	}
	return ""
}
