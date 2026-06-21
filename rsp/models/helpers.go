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

type fillData struct {
	fsrc float32
	ssrc string
	fdst *time.Duration
	sdst *time.Time
	recurse []Fillable
}

func f[T Fillable](fs []T) []Fillable {
	res := make([]Fillable, len(fs))
	for i, m := range fs {
		res[i] = m
	}
	return res
}

func fill(pairs []fillData) error {
	for _, p := range pairs {
		if p.ssrc != "" {
			if err := fillTime(p.ssrc, p.sdst); err != nil {
				return err
			}
		} else if len(p.recurse) > 0 {
			for _, r := range p.recurse {
				if err := r.Fill(); err != nil {
					return err
				}
			}
		} else {
			if err := fillDuration(p.fsrc, p.fdst); err != nil {
				return err
			}
		}
	}
	return nil
}

func fillTime(ts string, dest *time.Time) error {
	if ts == "" {
		return nil
	}
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
