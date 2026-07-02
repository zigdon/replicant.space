package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	lg "charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

func log(tmpl string, args ...any) {
	if !strings.HasSuffix(tmpl, "\n") {
		tmpl += "\n"
	}
	fmt.Fprintf(os.Stderr, tmpl, args...)
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

func f(n float32) string {
	return fmt.Sprintf("%.2f", n)
}

func d(n int) string {
	return fmt.Sprintf("%d", n)
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
	return fmt.Sprintf("%.0f%%", per)
}

func v(data any) string {
	s, _ := json.MarshalIndent(data, "", "  ")
	return string(s)
}

func dt(t time.Duration) string {
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
		return strings.Join(bits, "")
	} else {
		return t.String()
	}
}

func t(ts time.Time) string {
	var eta string
	if ts.Before(time.Now()) {
		eta = fmt.Sprintf("%s ago", dt(time.Since(ts)))
	} else {
		eta = fmt.Sprintf("in %s", dt(time.Until(ts)))
	}
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
	lg.Println(t)
}
