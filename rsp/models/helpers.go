package models

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	"github.com/rivo/tview"
	"github.com/zigdon/rsp/cache"
)

var db *cache.Cache

type Fillable interface {
	Fill() error
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

func log(tmpl string, args ...any) {
	fmt.Printf(time.Now().Format(time.Stamp)+" - "+tmpl+"\n", args...)
}

type Cachable interface {
	Cache() error
	Get() error
}

type LocalMsg interface {
	Notification() *Notification
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
	if c, ok := any(s).(Cachable); ok {
		if err := c.Cache(); err != nil {
			return s, fmt.Errorf("failed to update cache for %T: %v", s, err)
		}
	}
	if n, ok := any(s).(LocalMsg); ok {
		if err := n.Notification().Save(); err != nil {
			return s, fmt.Errorf("failed to create notification from %v: %v", s, err)
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
	if seconds <= 0 {
		return nil
	}
	var td time.Duration
	err := fillDuration(seconds, &td)
	*jtd = JSONTimeDelta{seconds, td}
	return err
}

func (jtd *JSONTimeDelta) MarshalJSON() ([]byte, error) {
	if jtd == nil {
		return []byte{}, nil
	}
	return json.Marshal(jtd.String())
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
	if orig == "" {
		return nil
	}
	var ts time.Time
	err := fillTime(orig, &ts)
	*jt = JSONTime{orig, ts}
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
	if jt == nil {
		return time.Time{}
	}
	return jt.ts
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
	return a.alias[:strings.Index(a.alias, "-")]
}

func (a *CodeAlias) Num() int {
	if a.alias == a.orig {
		return 0
	}
	n, err := strconv.Atoi(a.alias[strings.Index(a.alias, "-")+1:])
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
	return json.Marshal(fmt.Sprintf("%s (%s)", a.alias, a.orig))
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
