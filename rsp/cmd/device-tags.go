package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage device tags",
}

var addTagCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a tag to a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := getString(cmd, "device")
		res, err := rest.UpdateTags(models.NewCodeAlias(id), rest.AddTag, args)
		if err != nil {
			return err
		}
		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(res)
			return nil
		}
		tags := res.Tags
		if len(tags) == 0 {
			tags = []string{"N/A"}
		}
		printTable([]string{"Device", "Tags"}, [][]any{{res, list(tags)}})
		return nil
	},
}

var delTagCmd = &cobra.Command{
	Use:               "del",
	Aliases:           []string{"remove"},
	ValidArgsFunction: completeDeviceTags,
	Short:             "Remove a tag from a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := getString(cmd, "device")
		res, err := rest.UpdateTags(models.NewCodeAlias(id), rest.DelTag, args)
		if err != nil {
			return err
		}
		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(res)
			return nil
		}
		tags := res.Tags
		if len(tags) == 0 {
			tags = []string{"N/A"}
		}
		printTable([]string{"Device", "Tags"}, [][]any{{res, list(tags)}})
		return nil
	},
}

var findTagsCmd = &cobra.Command{
	Use:   "find",
	Short: "Find devices with a given tag",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("A tag must be specified")
		}
		res, err := rest.GetTagged(args[0])
		if err != nil {
			return err
		}
		var details [][]any
		for _, d := range res.Devices {
			code := d.Code.Alias()
			code = lines([]string{code, unalias(code)})
			var totalCargo int
			var cargo []string
			for _, c := range d.Cargo {
				totalCargo += c.Quantity
				cargo = append(cargo, fmt.Sprintf("%d × %s", c.Quantity, c.ResourceType))
			}
			cargo = append([]string{fmt.Sprintf("%d/%d (%d%%)",
				totalCargo, d.CargoCapacity, 100*totalCargo/d.CargoCapacity)}, cargo...)
			details = append(details, []any{code, d.Type, d.Location,
				d.Status, d.ReplicantCode, lines(cargo),
			})
		}

		printTable(
			[]string{"Code", "Type", "Location", "Status", "Replicant", "Cargo"},
			details,
		)

		return nil
	},
}

var listTagsCmd = &cobra.Command{
	Use:   "list_tags",
	Short: "List all defined tags",
	RunE:  listTags,
}

func listTags(cmd *cobra.Command, args []string) error {
	rows, err := db.DB.Query(`
		SELECT COUNT(code), type, JSONB_ARRAY_ELEMENTS_TEXT( data->'tags') AS tags
		FROM json_devices
		GROUP BY type, tags`)
	if err != nil {
		return err
	}
	tags := make(map[string]map[string]int)
	var types []string
	var tagNames []string
	miningTags := make(map[string]bool)
	mine := getBool(cmd, "mine")
	for rows.Next() {
		var c int
		var ty, tg string
		if err := rows.Scan(&c, &ty, &tg); err != nil {
			return err
		}
		if !mine && strings.HasPrefix(tg, "mine-") {
			miningTags[tg] = true
			continue
		}
		if _, ok := tags[tg]; !ok {
			tagNames = append(tagNames, tg)
			tags[tg] = make(map[string]int)
		}
		if !slices.Contains(types, ty) {
			types = append(types, ty)
		}
		tags[tg][ty]++
	}
	if err := rows.Close(); err != nil {
		return err
	}
	slices.Sort(types)
	slices.Sort(tagNames)
	var data [][]any
	for _, tag := range tagNames {
		ds := tags[tag]
		line := []any{tag}
		for _, t := range types {
			if ds[t] > 0 {
				line = append(line, ds[t])
			} else {
				line = append(line, "")
			}
		}
		data = append(data, line)
	}
	ts := []string{"Tag"}
	for _, t := range types {
		a, err := db.AliasType(t)
		if err != nil {
			return err
		}
		ts = append(ts, a)
	}
	printTable(ts, data)
	if len(miningTags) > 0 {
		fmt.Printf("Skipped %d mining tags\n", len(miningTags))
	}
	return nil
}

func init() {
	rootCmd.AddCommand(findTagsCmd)

	deviceCmd.AddCommand(tagCmd)
	tagCmd.AddCommand(addTagCmd)
	tagCmd.AddCommand(delTagCmd)
	rootCmd.AddCommand(listTagsCmd)
	listTagsCmd.Flags().BoolP("mine", "m", false, "If set, show mining tags")
}
