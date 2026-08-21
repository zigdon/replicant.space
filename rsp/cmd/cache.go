package cmd

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/rest"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the sqlite cache",
}

var cacheInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create the db or update the schema",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := cache.Connect()
		if err != nil {
			return err
		}
		log("cache updated: %s", db.Stats())
		return nil
	},
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cache stats",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := cache.Connect()
		if err != nil {
			return err
		}
		log("cache stats: %s", db.Stats())
		return nil
	},
}

var updateSchemaCmd = &cobra.Command{
	Use:   "update-schema",
	Short: "Update the database schema",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := cache.Connect()
		if err != nil {
			return err
		}
		return db.UpdateSchema()
	},
}

var reloadStarsCmd = &cobra.Command{
	Use:   "reload-stars",
	Short: "Fetch the full star census to the cache",
	RunE:  reloadStars,
}

var resetUniverseCmd = &cobra.Command{
	Use:   "reset-universe",
	Short: "Clear all the universe cache tables",
	RunE:  resetUniverse,
}

var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage aliases",
}

var aliasAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new alias for a device type",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("Missing arguments: rsp alias add <type> <alias>")
		}
		t := args[0]
		if a := db.GetPrefixForType(t); a != "" {
			return fmt.Errorf("%q already has a prefix: %q", t, a)
		}
		a := args[1]
		if t := db.GetTypeForPrefix(a); t != "" {
			return fmt.Errorf("%q is already a prefix: %q", a, t)
		}

		return db.AddAliasType(a, t)
	},
}

var aliasRenameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Change the alias of a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("Missing arguments: rsp alias rename <old> <new>")
		}
		oldAlias, newAlias := args[0], args[1]
		_, err := db.DB.Exec("UPDATE aliases SET name = $1 WHERE name = $2", newAlias, oldAlias)
		if err != nil {
			return err
		}
		return nil
	},
}

var aliasListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the all alias types",
	RunE: func(cmd *cobra.Command, args []string) error {
		rows, err := db.DB.Query("SELECT type, prefix FROM alias_types ORDER BY type")
		if err != nil {
			return err
		}
		var data [][]any
		var errs []error
		for rows.Next() {
			var t, p string
			errs = append(errs, rows.Scan(&t, &p))
			data = append(data, []any{t, p})
		}
		errs = append(errs, rows.Close())
		if err := errors.Join(errs...); err != nil {
			return err
		}
		printTable([]string{"Type", "Prefix"}, data)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(cacheInitCmd)
	cacheInitCmd.Flags().Bool("create", false, "Be willing to create a new db if none is found")
	cacheCmd.AddCommand(reloadStarsCmd)
	cacheCmd.AddCommand(statsCmd)
	cacheCmd.AddCommand(updateSchemaCmd)

	cacheCmd.AddCommand(resetUniverseCmd)
	resetUniverseCmd.Flags().Bool("delete", false, "Confirm that all the data should be deleted")
	resetUniverseCmd.MarkFlagRequired("delete")

	cacheCmd.AddCommand(aliasCmd)
	aliasCmd.AddCommand(aliasAddCmd)
	aliasCmd.AddCommand(aliasRenameCmd)
	aliasCmd.AddCommand(aliasListCmd)

	cacheCmd.AddCommand(intentCmd)
	intentCmd.AddCommand(intentListCmd)
	intentListCmd.Flags().StringP("location", "l", "", "Filter by location")
	intentListCmd.Flags().BoolP("inventory", "i", false, "Show existing inventory columns")

	intentCmd.AddCommand(intentAddCmd)
	intentAddCmd.Flags().StringP("location", "l", "", "Location designation")
	intentAddCmd.Flags().StringSliceP("resources", "r", []string{}, "Resources to add, type:qty (repeatable)")
	_ = intentAddCmd.RegisterFlagCompletionFunc("resources", completeResources)
	_ = intentAddCmd.RegisterFlagCompletionFunc("location", completeStarsAndPlanets)

	intentCmd.AddCommand(intentRemoveCmd)
	intentRemoveCmd.Flags().StringP("location", "l", "", "Location designation")
	intentRemoveCmd.Flags().StringSliceP("resources", "r", []string{}, "Specific resources to remove (repeatable)")
	_ = intentRemoveCmd.RegisterFlagCompletionFunc("resources", completeResources)
	_ = intentRemoveCmd.RegisterFlagCompletionFunc("location", completeIntents)
}

var intentCmd = &cobra.Command{
	Use:   "intent",
	Short: "Manage resource intents",
	RunE:  intentListCmd.RunE,
}

var intentListCmd = &cobra.Command{
	Use:               "list [location]",
	Aliases:           []string{"ls"},
	Short:             "List resource intents and current inventory",
	ValidArgsFunction: completeIntents,
	RunE: func(cmd *cobra.Command, args []string) error {
		var loc string
		if len(args) > 0 {
			loc = args[0]
		}
		if l := getString(cmd, "location"); l != "" {
			loc = l
		}
		records, err := db.GetIntents(loc)
		if err != nil {
			return err
		}
		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(records)
			return nil
		}
		if len(records) == 0 {
			if loc != "" {
				log("No intents found for %s", loc)
			} else {
				log("No intents found")
			}
			return nil
		}

		showInv := getBool(cmd, "inventory")

		// Collect all resource types across records, keeping StandardResources order
		allRes := slices.Clone(StandardResources)
		for _, r := range records {
			for k := range r.Demand {
				if !slices.Contains(allRes, k) {
					allRes = append(allRes, k)
				}
			}
			if showInv {
				for k := range r.Inventory {
					if !slices.Contains(allRes, k) {
						allRes = append(allRes, k)
					}
				}
			}
		}

		var activeRes []string
		for _, res := range allRes {
			for _, r := range records {
				if r.Demand[res] > 0 || (showInv && r.Inventory[res] > 0) {
					activeRes = append(activeRes, res)
					break
				}
			}
		}
		if len(activeRes) == 0 {
			activeRes = slices.Clone(StandardResources)
		}

		headers := []string{"Location"}
		if showInv {
			for _, res := range activeRes {
				title := strings.ToUpper(res[:1]) + res[1:]
				headers = append(headers, title+" (Int)")
			}
			for _, res := range activeRes {
				title := strings.ToUpper(res[:1]) + res[1:]
				headers = append(headers, title+" (Inv)")
			}
		} else {
			for _, res := range activeRes {
				title := strings.ToUpper(res[:1]) + res[1:]
				headers = append(headers, title)
			}
		}

		var data [][]any
		for _, r := range records {
			row := []any{r.Location}
			for _, res := range activeRes {
				intentQty := r.Demand[res]
				invQty := r.Inventory[res]
				row = append(row, formatIntentCell(intentQty, invQty))
			}
			if showInv {
				for _, res := range activeRes {
					intentQty := r.Demand[res]
					invQty := r.Inventory[res]
					row = append(row, formatInventoryCell(intentQty, invQty))
				}
			}
			data = append(data, row)
		}
		printTable(headers, data)
		return nil
	},
}

func formatIntentCell(intentQty, invQty int) string {
	if intentQty <= 0 {
		return ""
	}
	if invQty < intentQty {
		diff := invQty - intentQty
		return fmt.Sprintf("%d (%d)", intentQty, diff)
	}
	return fmt.Sprintf("%d", intentQty)
}

func formatInventoryCell(intentQty, invQty int) string {
	if invQty > 0 {
		return fmt.Sprintf("%d", invQty)
	}
	if intentQty > 0 {
		return "0"
	}
	return ""
}

var intentAddCmd = &cobra.Command{
	Use:               "add <location> <resource:qty>...",
	Short:             "Add or update resource intent for a location",
	ValidArgsFunction: completeIntentAddArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var loc string
		var resArgs []string
		locFlag := getString(cmd, "location")
		if locFlag != "" {
			loc = locFlag
			resArgs = args
		} else if len(args) > 0 {
			loc = args[0]
			resArgs = args[1:]
		}

		if loc == "" {
			return fmt.Errorf("Missing arguments: rsp cache intent add <location> <resource:qty>...")
		}

		resSlice := getStringSlice(cmd, "resources")
		demand := make(map[string]int)

		for _, r := range resSlice {
			if r == "" {
				continue
			}
			k, v, ok := strings.Cut(r, ":")
			if !ok {
				k, v, ok = strings.Cut(r, "=")
			}
			if !ok {
				return fmt.Errorf("Invalid resource format %q: expected resource:qty", r)
			}
			qty, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return fmt.Errorf("Invalid quantity in %q: %w", r, err)
			}
			if qty < 0 {
				return fmt.Errorf("Quantity cannot be negative: %d", qty)
			}
			demand[strings.ToLower(strings.TrimSpace(k))] = qty
		}

		for i := 0; i < len(resArgs); i++ {
			arg := resArgs[i]
			if strings.Contains(arg, ":") || strings.Contains(arg, "=") {
				k, v, ok := strings.Cut(arg, ":")
				if !ok {
					k, v, _ = strings.Cut(arg, "=")
				}
				qty, err := strconv.Atoi(strings.TrimSpace(v))
				if err != nil {
					return fmt.Errorf("Invalid quantity in %q: %w", arg, err)
				}
				if qty < 0 {
					return fmt.Errorf("Quantity cannot be negative: %d", qty)
				}
				demand[strings.ToLower(strings.TrimSpace(k))] = qty
			} else {
				if i+1 < len(resArgs) {
					qty, err := strconv.Atoi(strings.TrimSpace(resArgs[i+1]))
					if err == nil {
						if qty < 0 {
							return fmt.Errorf("Quantity cannot be negative: %d", qty)
						}
						demand[strings.ToLower(strings.TrimSpace(arg))] = qty
						i++
						continue
					}
				}
				return fmt.Errorf("Invalid resource argument %q: expected resource:qty (e.g. carbon:500) or resource qty (e.g. carbon 500)", arg)
			}
		}

		if len(demand) == 0 {
			return fmt.Errorf("At least one resource requirement must be specified (e.g. carbon:500)")
		}

		if err := db.AddIntent(loc, demand); err != nil {
			return err
		}
		log("Added intent for %s: %v", loc, demand)
		return nil
	},
}

var intentRemoveCmd = &cobra.Command{
	Use:               "remove <location> [resource...]",
	Aliases:           []string{"rm", "del", "delete"},
	Short:             "Remove an intent or specific resources from a location's intent",
	ValidArgsFunction: completeIntentRemoveArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var loc string
		var resArgs []string
		locFlag := getString(cmd, "location")
		if locFlag != "" {
			loc = locFlag
			resArgs = args
		} else if len(args) > 0 {
			loc = args[0]
			resArgs = args[1:]
		}

		if loc == "" {
			return fmt.Errorf("Missing arguments: rsp cache intent remove <location> [resource...]")
		}

		resSlice := getStringSlice(cmd, "resources")
		var resources []string
		for _, r := range resSlice {
			if r != "" {
				k, _, _ := strings.Cut(r, ":")
				k, _, _ = strings.Cut(k, "=")
				resources = append(resources, strings.ToLower(strings.TrimSpace(k)))
			}
		}
		for _, arg := range resArgs {
			k, _, _ := strings.Cut(arg, ":")
			k, _, _ = strings.Cut(k, "=")
			resources = append(resources, strings.ToLower(strings.TrimSpace(k)))
		}

		if err := db.RemoveIntent(loc, resources); err != nil {
			return err
		}
		if len(resources) > 0 {
			log("Removed %v from intent for %s", resources, loc)
		} else {
			log("Removed intent for %s", loc)
		}
		return nil
	},
}

func resetUniverse(cmd *cobra.Command, args []string) error {
	for _, t := range []cache.Tables{
		cache.StarsTable,
		cache.PlanetsTable,
		cache.MoonsTable,
		cache.BeltsTable,
		cache.BlueprintsTable,
	} {
		if err := db.Reset(t); err != nil {
			return fmt.Errorf("Couldn't clear %s: %v", t, err)
		}
	}
	return nil
}

func reloadStars(cmd *cobra.Command, args []string) error {
	out, err := rest.ReloadStars()
	if err != nil {
		return err
	}
	log(out)
	return nil
}
