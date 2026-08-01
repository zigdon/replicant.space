package common

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	lg "charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

func PrintTable(headers []string, data [][]any) {
	PrintTablef(os.Stdout, headers, data)
}

func PrintTablef(out io.Writer, headers []string, inData [][]any) {
	var cellStyles []lg.Style
	headerStyle := lg.NewStyle().Bold(true).Align(lg.Center)
	cellStyle := lg.NewStyle().Padding(0, 1)
	cols := len(headers)
	if cols == 0 {
		cols = len(inData[0])
	}
	hasData := make([]bool, cols)
	var data [][]string
	for _, r := range inData {
		var line []string
		for _, c := range r {
			line = append(line, stringify(c))
		}
		data = append(data, line)
	}
	for i := range cols {
		var max int
		if len(headers) > 0 {
			max = len(headers[i])
		}
		for _, l := range data {
			item := l[i]
			if len(item) > 0 && l[i] != "0" && l[i] != "0.00" {
				hasData[i] = true
			}
			if strings.Contains(item, "\n") {
				for nl := range strings.SplitSeq(item, "\n") {
					if len(nl) > max {
						max = len(nl)
					}
				}
			} else {
				if len(item) > max {
					max = len(item)
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

func f(n float32) string {
	return humanize(fmt.Sprintf("%.2f", n))
}

func d(n int) string {
	return humanize(fmt.Sprintf("%d", n))
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

func humanize(in string) string {
	if strings.Contains(in, ",") {
		return in
	}
	var out string
	var neg bool
	if in[0] == '-' {
		neg = true
		in = in[1:]
	}
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
	if neg {
		return "-" + out
	}
	return out
}

func T(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	eta := Dt(time.Until(ts))
	return Lines([]string{
		ts.Format(time.DateTime), eta,
	})
}

func Dt(t time.Duration) string {
	if t == 0 {
		return ""
	}
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

func Lines(s []string) string {
	return strings.Join(s, "\n")
}
