package models

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	"github.com/rivo/tview"
	"github.com/zigdon/rsp/cache"
)

var db *cache.Cache

func psqlDuration(in string) (time.Duration, error) {
	in = strings.Replace(in, ":", "h", 1)
	in = strings.Replace(in, ":", "m", 1)
	in += "s"
	return time.ParseDuration(in)
}

type Fillable interface {
	Fill() error
}

func fill[T []E, E Fillable](s []E) error {
	var errs []error
	for _, e := range s {
		errs = append(errs, e.Fill())
	}
	return errors.Join(errs...)
}

func fillTime(ts string, dest *time.Time) error {
	if ts == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return err
	}
	*dest = parsed.Truncate(time.Second)
	return nil
}

type Cachable interface {
	Cache() error
	Get() error
}

func cacheItems[T []E, E Cachable](s []E) error {
	var errs []error
	for _, e := range s {
		errs = append(errs, e.Cache())
	}
	return errors.Join(errs...)
}

type LocalMsg interface {
	Notification() *Notification
}

func Parse[T any](data []byte) (*T, error) {
	s := new(T)
	var errs []error

	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("Error parsing %T: %v\n%s", s, err, string(data))
	}
	if f, ok := any(s).(Fillable); ok {
		errs = append(errs, f.Fill())
	}
	if c, ok := any(s).(Cachable); ok {
		errs = append(errs, c.Cache())
	}
	if n, ok := any(s).(LocalMsg); ok {
		errs = append(errs, n.Notification().Save())
	}

	return s, errors.Join(errs...)
}

func ConnectDB(cdb *cache.Cache) {
	db = cdb
}

type JSONTimeDelta struct {
	seconds float32
	td      time.Duration
}

func (jtd *JSONTimeDelta) UnmarshalJSON(data []byte) error {
	var tds string
	var tdf float32
	var td time.Duration
	if err := json.Unmarshal(data, &tds); err == nil {
		td, err = time.ParseDuration(tds)
		if err != nil {
			return err
		}
	} else if err := json.Unmarshal(data, &tdf); err == nil {
		td, err = time.ParseDuration(fmt.Sprintf("%.2fs", tdf))
		if err != nil {
			return err
		}
	} else {
		return err
	}
	*jtd = JSONTimeDelta{float32(td.Seconds()), td.Truncate(time.Second)}
	return nil
}

func (jtd *JSONTimeDelta) MarshalJSON() ([]byte, error) {
	if jtd == nil {
		return []byte{}, nil
	}
	return json.Marshal(jtd.td.Seconds())
}

func (jtd *JSONTimeDelta) String() string {
	if jtd == nil {
		return ""
	}
	return jtd.td.String()
}

func (jtd *JSONTimeDelta) Duration() time.Duration {
	if jtd == nil {
		return 0
	}
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
	if orig == "" {
		return nil
	}
	var ts time.Time
	err := fillTime(orig, &ts)
	*jt = JSONTime{orig, ts.Truncate(time.Second)}
	return err
}

func (jt *JSONTime) MarshalJSON() ([]byte, error) {
	if jt == nil {
		return []byte{}, nil
	}
	return json.Marshal(jt.String())
}

func (jt *JSONTime) String() string {
	if jt == nil {
		return ""
	}
	return jt.ts.Format(time.RFC3339)
}

func (jt *JSONTime) Format() string {
	if jt == nil {
		return ""
	}
	now := time.Now()
	var eta string
	if jt.ts.Before(now) {
		eta = fmt.Sprintf("%s ago", now.Sub(jt.ts).Truncate(time.Second).String())
	} else {
		eta = fmt.Sprintf("in %s", jt.ts.Sub(now).Truncate(time.Second).String())
	}
	return fmt.Sprintf("%s (%s)", jt.ts.Format(time.Stamp), eta)
}

func (jt *JSONTime) Time() time.Time {
	if jt == nil {
		return time.Time{}
	}
	return jt.ts
}

func (jt *JSONTime) Set(ts time.Time) *JSONTime {
	if jt == nil {
		return &JSONTime{ts: ts}
	}
	jt.ts = ts
	return jt
}

func NewCodeAlias(input string) *CodeAlias {
	c := &CodeAlias{}
	if strings.Contains(input, "-") {
		c.alias = input
		c.orig = db.Dealias(input)
	} else {
		c.orig = input
		alias, err := db.Alias(input, "")
		if err == nil {
			c.alias = alias
		}
	}
	return c
}

func CompareAliases(a, b *CodeAlias) int {
	return cmp.Or(
		cmp.Compare(a.Type(), b.Type()),
		cmp.Compare(a.Num(), b.Num()),
	)
}

type CodeAlias struct {
	orig  string
	alias string
}

func (a *CodeAlias) Type() string {
	if a.alias == a.orig {
		return ""
	}
	t, _, _ := strings.Cut(a.alias, "-")
	return t
}

func (a *CodeAlias) Num() int {
	if a.alias == a.orig {
		return 0
	}
	_, id, _ := strings.Cut(a.alias, "-")
	n, err := strconv.Atoi(id)
	if err != nil {
		fmt.Printf("Failed to get number of %q: %v\n", a.alias, err)
		return 0
	}
	return n
}

func (a *CodeAlias) MarshalJSON() ([]byte, error) {
	if a == nil {
		return []byte{}, nil
	}
	return json.Marshal(a.orig)
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

func (a *CodeAlias) Contained(l []*CodeAlias) bool {
	for _, i := range l {
		if a.orig == i.orig {
			return true
		}
	}
	return false
}

type LocationID string

func (l LocationID) Star() string {
	star, _, ok := strings.Cut(string(l), "-")
	if ok {
		return star
	}
	return string(l)
}

func TreeNode(tmpl string, args ...any) *tview.TreeNode {
	return tview.NewTreeNode(fmt.Sprintf(" "+tmpl, args...))
}

func ref[T any](s T) func() []any {
	return func() []any {
		return []any{s}
	}
}

type UpdateFn struct {
	Tmpl    string
	ArgFn   func() []any
	TextFn  func() string
	ChildFn func() []string
}

func TreeNodeFn(tmpl string, fn func() []any) *tview.TreeNode {
	return tview.NewTreeNode("").
		SetText(fmt.Sprintf(" "+tmpl, fn()...)).
		SetReference(UpdateFn{
			Tmpl:  tmpl,
			ArgFn: fn,
		})
}

func TreeNodeGen(label string, fn func() []string) *tview.TreeNode {
	tn := tview.NewTreeNode(label).
		SetReference(UpdateFn{
			Tmpl:    label,
			ChildFn: fn,
		})
	return tn
}

func ProgressTime(width int, start, end time.Time) string {
	total := end.Sub(start)
	now := time.Now()
	prog := now.Sub(start)
	pct := prog.Seconds() / total.Seconds()
	cnt := int(pct * float64(width))
	return fmt.Sprintf("%s%s %s %.0f%%",
		strings.Repeat("⬜", cnt),
		strings.Repeat("⬛", width-cnt),
		end.Sub(now).Round(time.Millisecond).String(),
		100*pct)
}

type DeviceFilter func(*Device) bool

func DeviceFilterTags(reject, require []string) DeviceFilter {
	mustHave := make(map[string]bool)
	for _, t := range require {
		mustHave[t] = true
	}
	mustNotHave := make(map[string]bool)
	for _, t := range reject {
		mustNotHave[t] = true
	}
	return func(d *Device) bool {
		var pass bool
		if len(mustHave) == 0 {
			pass = true
		}
		for _, t := range d.Tags {
			if mustNotHave[t] {
				return false
			}
			if mustHave[t] {
				pass = true
			}
		}
		return pass
	}
}

func DeviceFilterMatrix() DeviceFilter {
	return func(d *Device) bool {
		return !strings.Contains(d.Type, "matrix") || (d.Status != "stowed" && d.Status != "idle")
	}
}

func DeviceFilterMine() DeviceFilter {
	return func(d *Device) bool {
		return len(d.Tags) == 0 || slices.ContainsFunc(
			d.Tags, func(s string) bool {
				// No location, e.g. in transit, keep
				if d.Location == "" {
					return true
				}
				// If the location doesn't match the mining tag, keep
				loc := strings.ToLower(string(d.Location))
				return s != fmt.Sprintf("mine-%s", loc)
			})
	}
}
