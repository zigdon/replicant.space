package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	lg "charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/rivo/tview"
)

var LogFh io.Writer = os.Stderr

func newLogWindow() *tview.TextView {
	lw := tview.NewTextView()
	lw.SetBorder(true).SetTitle(" Log ")
	lw.SetChangedFunc(func() {
		lw.ScrollToEnd()
	})
	LogFh = lw
	return lw
}

func log(tmpl string, args ...any) {
	date := time.Now().Format(time.DateTime) + " - "
	if !strings.HasSuffix(tmpl, "\n") {
		tmpl += "\n"
	}
	fmt.Fprintf(LogFh, date+tmpl, args...)
}

func die(tmpl string, args ...any) {
	log("FATAL: "+tmpl, args...)
	os.Exit(1)
}

func prettyPrint(i any) {
	s, _ := json.MarshalIndent(i, "", "  ")
	fmt.Println(string(s))
}

func wrap(t string, w int) string {
	return lg.NewStyle().Width(w).Render(t)
}

func b(n bool) string {
	if n {
		return "True"
	}
	return "False"
}

func humanize(in string) string {
	var out string
	d := strings.Index(in, ".")
	if d >= 0 {
		out = in[d:]
		in = in[:d]
	}
	for len(in) > 3 {
		out = "," + in[len(in)-3:] + out
		in = in[:len(in)-3]
	}
	out = in + out
	return out
}

func f(n float32) string {
	return humanize(fmt.Sprintf("%.2f", n))
}

func d(n int) string {
	return humanize(fmt.Sprintf("%d", n))
}

func list(s []string) string {
	return strings.Join(s, ", ")
}

func lines(s []string) string {
	return strings.Join(s, "\n")
}

func m[T any](in map[string]T) string {
	var res []string
	for k, v := range in {
		res = append(res, fmt.Sprintf("%s: %v", k, v))
	}
	return strings.Join(res, "\n")
}

func p(per float32) string {
	if per == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f%%", per)
}

func v(data any) string {
	if data == nil {
		return ""
	}
	s, _ := json.MarshalIndent(data, "", "  ")
	if string(s) == "null" {
		return ""
	}
	return string(s)
}

func dt(t time.Duration) string {
	tmpl := "in %s"
	if t < 0 {
		tmpl = "%s ago"
	}
	t = t.Abs().Round(time.Second)
	if t > 24*time.Hour {
		t = t.Round(time.Minute)
		bits := []string{fmt.Sprintf("%.0fd", t.Hours()/24)}
		t %= 24 * time.Hour
		if t.Hours() >= 1 {
			bits = append(bits, fmt.Sprintf("%.0fh", t.Hours()))
		}
		t %= time.Hour
		if t.Minutes() >= 1 {
			bits = append(bits, fmt.Sprintf("%.0fm", t.Minutes()))
		}
		return fmt.Sprintf(tmpl, strings.Join(bits, ""))
	} else {
		return fmt.Sprintf(tmpl, t.String())
	}
}

func t(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	eta := dt(time.Until(ts))
	return lines([]string{
		ts.Format(time.DateTime), eta,
	})
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

func printTable(headers []string, data [][]string) {
	printTablef(os.Stdout, headers, data)
}

func printTablef(out io.Writer, headers []string, data [][]string) {
	var cellStyles []lg.Style
	headerStyle := lg.NewStyle().Bold(true).Align(lg.Center)
	cellStyle := lg.NewStyle().Padding(0, 1)
	cols := len(headers)
	if cols == 0 {
		cols = len(data[0])
	}
	hasData := make([]bool, cols)
	for i := range cols {
		var max int
		if len(headers) > 0 {
			max = len(headers[i])
		}
		for _, l := range data {
			if len(l[i]) > 0 {
				hasData[i] = true
			}
			if strings.Contains(l[i], "\n") {
				for nl := range strings.SplitSeq(l[i], "\n") {
					if len(nl) > max {
						max = len(nl)
					}
				}
			} else {
				if len(l[i]) > max {
					max = len(l[i])
				}
			}
		}
		cellStyles = append(cellStyles, cellStyle.Width(max+2))
	}

	headers = filterEmpty(headers, hasData)
	cellStyles = filterEmpty(cellStyles, hasData)
	for i, l := range data {
		data[i] = filterEmpty(l, hasData)
	}

	t := table.New().
		Border(lg.NormalBorder()).
		StyleFunc(func(row, col int) lg.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyles[col]
		}).
		Headers(headers...).
		Rows(data...)
	lg.Fprintln(out, t)
}
