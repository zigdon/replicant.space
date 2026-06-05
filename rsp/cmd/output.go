package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

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

func m[T int | string](in map[string]T) string {
	var res []string
	for k, v := range in {
		res = append(res, fmt.Sprintf("%s: %v", k, v))
	}
	return strings.Join(res, "\n")
}

func p(per float32) string {
	return fmt.Sprintf("%.2f%%", per)
}

func v(data any) string {
	s, _ := json.MarshalIndent(data, "", "  ")
	return string(s)
}

func printTable(headers []string, data [][]string) {
	var cellStyles []lg.Style
	headerStyle := lg.NewStyle().Bold(true).Align(lg.Center)
	cellStyle := lg.NewStyle().Padding(0, 1)
	for i := range headers {
		max := len(headers[i])
		for _, l := range data {
			if strings.Contains(l[i], "\n") {
				for _, nl := range strings.Split(l[i], "\n") {
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
