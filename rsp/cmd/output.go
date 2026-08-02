package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	lg "charm.land/lipgloss/v2"
	"github.com/rivo/tview"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
)

func newLogWindow() *tview.TextView {
	lw := tview.NewTextView()
	lw.SetBorder(true).SetTitle(" Log ")
	lw.SetChangedFunc(func() {
		lw.ScrollToEnd()
	})
	common.LogFh = lw
	return lw
}

func log(tmpl string, args ...any) {
	common.Log(tmpl, args...)
}

func die(tmpl string, args ...any) {
	log("FATAL: "+tmpl, args...)
	os.Exit(1)
}

func prettyPrint(i any) {
	prettyPrintf(os.Stdout, i)
}

func prettyPrintf(f io.Writer, i any) {
	s, _ := json.MarshalIndent(i, "", "  ")
	fmt.Fprintln(f, string(s))
}

func codeList(cs []*models.CodeAlias) []string {
	res := make([]string, len(cs))
	for i, c := range cs {
		res[i] = c.Alias()
	}
	return res
}

func devList(cs []*models.Device) []string {
	res := make([]string, len(cs))
	for i, c := range cs {
		res[i] = c.Code.Alias()
	}
	return res
}

func wrap(t string, w int) string {
	return lg.NewStyle().Width(w).Render(t)
}

func list(s []string) string {
	res := strings.Join(s, ", ")
	if len(res) > 30 {
		res = fmt.Sprintf("(%d) %s...", len(s), res[:30])
	}
	return res
}

func lines(s []string) string {
	return common.Lines(s)
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

func rm(data map[string]int) string {
	var h, d string
	for _, r := range []string{"carbon", "conductive", "rares", "silicates",
		"structural", "volatiles"} {
		if data[r] <= 0 {
			continue
		}
		h += strings.ToUpper(r[:1]) + r[1:3]
		d += fmt.Sprintf("%3d ", data[r])
		if len(d) > len(h) {
			h += strings.Repeat(" ", len(d)-len(h))
		}
	}
	var res []string
	if len(h) > 0 {
		res = []string{h, d}
	}
	for k, v := range data {
		if isResource(k) {
			continue
		}
		res = append(res, fmt.Sprintf("%s: %d", k, v))
	}
	return strings.Join(res, "\n")
}

func printTable(headers []string, data [][]any) {
	common.PrintTable(headers, data)
}

func printTablef(out io.Writer, headers []string, inData [][]any) {
	common.PrintTablef(out, headers, inData)
}

func t(in time.Time) string {
	return common.T(in)
}
