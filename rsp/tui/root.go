package tui

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/zigdon/rsp/models"
)

var devList = tview.NewList()
var tree = tview.NewTreeView().SetRoot(tview.NewTreeNode("Details"))
var dump = tview.NewTextView()
var logWin = tview.NewTextView()
var app *tview.Application
var c = newCache()

var TUI = &cobra.Command{
	Use: "tui",
	Short: "Launch a TUI interface",
	RunE: func(cmd *cobra.Command, args []string) error {
		app = tview.NewApplication()
		go processEventQueue()
		app.SetRoot(layout(), true)
		if err := replPage(); err != nil {
			return err
		}
		setKeys()
		Repeat("update tree", 5*time.Second, func() error {
			update(tree.GetRoot())
			return nil
		}, forever)
		return app.Run()
	},
}

func log(tmpl string, args ...any) {
	now := time.Now().Format(time.Stamp)
	args = append([]any{now}, args...)
	if app.GetFocus() != nil {
		fmt.Fprintf(logWin, "%s: " + tmpl + "\n", args...)
		if !logWin.HasFocus() {
			logWin.ScrollToEnd()
		}
	} else {
		fmt.Printf("%s: " + tmpl + "\n", args...)
	}
}

func setKeys() {
	app.SetInputCapture(func (ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Rune() == 'l' && ev.Modifiers()&tcell.ModAlt != 0:
			app.SetFocus(logWin)
			return nil
		}
		return ev
	})
}

func layout() *tview.Flex {
	logWin.SetDoneFunc(func(tcell.Key) {
		app.SetFocus(devList)
	})

	dump.SetDoneFunc(func(tcell.Key) {
		app.SetFocus(devList)
	})

	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewFlex().
		AddItem(devList, 0, 1, true).
		AddItem(tview.NewFlex().
			AddItem(tree, 0, 1, false).
			AddItem(dump, 0, 1, false),
			0, 5, false), 0, 1, true).
		AddItem(logWin, 10, 0, false)
}

func update(tn *tview.TreeNode) {
	if len(tn.GetChildren()) > 0 {
		for _, c := range tn.GetChildren() {
			update(c)
		}
	}
	r := tn.GetReference()
	if r == nil {
		return
	}
	uf, ok := r.(models.UpdateFn)
	if !ok {
		log("Can't use %v as UpdateFn", r)
		return
	}
	if uf.ArgFn != nil {
		tn.SetText(fmt.Sprintf(uf.Tmpl, uf.ArgFn()...))
	} else if uf.TextFn != nil {
		tn.SetText(uf.TextFn())
	} else {
		tn.SetText(uf.Tmpl)
	}
	if uf.ChildFn != nil {
		tn.ClearChildren()
		for _, c := range uf.ChildFn() {
			tn.AddChild(tview.NewTreeNode(c))
		}
		return
	}
}

func replPage() error {
	if err := c.update(); err != nil {
		return err
	}

	devList.Clear()
	var reps []*models.Replicant
	for _, r := range c.getAll("replicant") {
		reps = append(reps, r.(*models.Replicant))
	}
	slices.SortFunc(reps, func(a, b *models.Replicant) int {
		return cmp.Compare(a.Code.Alias(), b.Code.Alias())
	})
	for _, rep := range reps {
		m, s := rep.ListItem()
		devList.AddItem(m, s, 0, func() {
			app.SetFocus(dump)
		})
	}
	devList.SetChangedFunc(func(i int, _, _ string, _ rune) {
		rep := c.c[reps[i].Code.String()].(*models.Replicant)
		pp, _ := json.MarshalIndent(rep, "", "  ")
		dump.SetText(string(pp))
		tree.GetRoot().ClearChildren()
		for _, tn := range rep.Details() {
			tree.GetRoot().AddChild(tn)
		}
		update(tree.GetRoot())
	})
	devList.SetCurrentItem(1)
	devList.SetCurrentItem(0)

	return nil
}
