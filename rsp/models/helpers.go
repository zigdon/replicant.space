package models

import (
	"fmt"
	"time"

	"encoding/json"

	"github.com/rivo/tview"
	"github.com/zigdon/rsp/cache"
)

var db *cache.Cache

type Fillable interface {
	Fill() error
}

type fillData struct {
	fsrc    float32
	ssrc    string
	fdst    *time.Duration
	sdst    *time.Time
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
		if p.sdst != nil {
			if err := fillTime(p.ssrc, p.sdst); err != nil {
				return err
			}
		} else if len(p.recurse) > 0 {
			for _, r := range p.recurse {
				if err := r.Fill(); err != nil {
					return err
				}
			}
		} else if p.fdst != nil {
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

type JSONTimeDelta struct {
	seconds float32
	td      time.Duration
}

func (jtd *JSONTimeDelta) UnmarshalJSON(data []byte) error {
	var seconds float32
	if err := json.Unmarshal(data, &seconds); err != nil {
		return err
	}
	var td time.Duration
	err := fillDuration(seconds, &td)
	*jtd = JSONTimeDelta{seconds, td}
	return err
}

func (jtd *JSONTimeDelta) String() string {
	if jtd == nil {
		return ""
	}
	return jtd.td.String()
}

func (jtd *JSONTimeDelta) Duration() time.Duration {
	return jtd.td
}

type JSONTime struct {
	orig string
	ts   time.Time
}

func (jt *JSONTime) UnmarshalJSON(data []byte) error {
	var orig string
	if err := json.Unmarshal(data, &orig); err != nil {
		return err
	}
	var ts time.Time
	err := fillTime(orig, &ts)
	*jt = JSONTime{orig, ts}
	return err
}

func (jt *JSONTime) String() string {
	if jt == nil {
		return ""
	}
	now := time.Now()
	var eta string
	if jt.ts.Before(now) {
		eta = fmt.Sprintf("%s ago", now.Sub(jt.ts).Round(time.Second).String())
	} else {
		eta = fmt.Sprintf("in %s", jt.ts.Sub(now).Round(time.Second).String())
	}
	return fmt.Sprintf("%s (%s)", jt.ts.Format(time.DateTime), eta)
}

func (jt *JSONTime) Time() time.Time {
	return jt.ts
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

func (a *CodeAlias) Alias() string {
	if a != nil {
		if a.alias != "" {
			return a.alias
		}
		return a.orig
	}
	return ""
}

func TreeNode(tmpl string, args ...any) *tview.TreeNode {
	return tview.NewTreeNode(fmt.Sprintf(tmpl, args...))
}
