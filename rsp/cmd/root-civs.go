package cmd

import (
	"cmp"
	"slices"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var civCmd = &cobra.Command{
	Use: "species",
	Aliases: []string{"civs", "civilisations", "civ"},
	Short: "List known species",
	RunE: civ,
}

func civ(cmd *cobra.Command, args []string) error {
	res, err := rest.Species()
	if err != nil {
		return err
	}

	var data [][]string
	for _, s := range res.Species {
		data = append(data, []string{
			s.Name, wrap(s.Greeting, 30), wrap(s.Description, 40),
			wrap(s.Government, 20), s.HomeworldType, s.TechAffinity, s.Trait,
		})
	}
	slices.SortFunc(data, func(a, b []string) int {
		return cmp.Compare(a[0], b[0])
	})
	printTable([]string{
			"Name", "Greeting", "Description", "Government", "Homeworld", "Tech", "Traits"},
			data)
	return nil
}

func init() {
	rootCmd.AddCommand(civCmd)
}
