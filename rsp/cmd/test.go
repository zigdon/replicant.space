package cmd

import (
	"fmt"
	"maps"

	"github.com/spf13/cobra"
)

type globalResourceMap struct {
	m map[string]map[string]int
}

func newGRM() *globalResourceMap {
	return &globalResourceMap{
		m: make(map[string]map[string]int),
	}
}

func (grm *globalResourceMap) Add(loc, res string, qty int) int {
	if _, ok := grm.m[loc]; !ok {
		grm.m[loc] = make(map[string]int)
	}
	grm.m[loc][res] += qty

	return grm.m[loc][res]
}

func (grm *globalResourceMap) Set(loc, res string, qty int) int {
	if _, ok := grm.m[loc]; !ok {
		grm.m[loc] = make(map[string]int)
	}
	grm.m[loc][res] = qty

	return grm.m[loc][res]
}

func (grm *globalResourceMap) Get(loc string) map[string]int {
	if _, ok := grm.m[loc]; !ok {
		return nil
	}
	ret := make(map[string]int)
	maps.Copy(ret, grm.m[loc])
	return ret
}

func (grm *globalResourceMap) Clear() {
	clear(grm.m)
}

func (grm *globalResourceMap) Dump() {
	prettyPrint(grm.m)
}

func (grm *globalResourceMap) Clone() *globalResourceMap {
	ngrm := newGRM()
	for k, v := range grm.m {
		maps.Copy(ngrm.m[k], v)
	}
	return ngrm
}

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newGRM()
		fmt.Println(m.Add("locA", "resA", 10))
		fmt.Println(m.Add("locA", "resA", 10))
		fmt.Println(m.Get("locB"))
		m.Dump()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
