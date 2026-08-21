package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/rest"
)

func completeStarsAndPlanets(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	res, dir := completeStars(cmd, args, toComplete)
	p, _ := completePlanets(cmd, args, toComplete)
	res = append(res, p...)

	return res, dir
}

func completeStars(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var res []string
	if len(toComplete) < 3 {
		q := `
		  SELECT * FROM (
		    SELECT DISTINCT(SUBSTR(designation, 0, 4)) AS prefix, COUNT(designation) AS cnt
			FROM stars
			GROUP BY prefix
		  ) WHERE prefix LIKE $1
		  ORDER BY prefix
		`
		rows, err := db.DB.Query(q, strings.ToUpper(toComplete)+"%")
		if err != nil {
			log("Can't query prefix %q: %v", toComplete, err)
			return res, cobra.ShellCompDirectiveNoFileComp
		}
		var total int
		var prefixes []string
		for rows.Next() {
			var prefix string
			var cnt int
			if err := rows.Scan(&prefix, &cnt); err == nil {
				prefixes = append(prefixes, fmt.Sprintf("%s\t%d stars", prefix, cnt))
				total += cnt
			}
		}
		if total > 500 {
			return prefixes, cobra.ShellCompDirectiveNoFileComp
		}
	}

	rows, err := db.DB.Query(`
		SELECT designation, explored, has_life, entry_point
		FROM stars
		WHERE starts_with(designation, $1)
	`, strings.ToUpper(toComplete))
	if err != nil {
		log("Can't query prefix %q: %v", toComplete, err)
		return res, cobra.ShellCompDirectiveNoFileComp
	}
	for rows.Next() {
		var d, ep string
		var e, l bool
		if err := rows.Scan(&d, &e, &l, &ep); err != nil {
			continue
		}
		res = append(res, fmt.Sprintf("%s\tExplored: %v, Has life: %v", d, e, l))
		res = append(res, fmt.Sprintf("%s\tEntry point", ep))
	}

	return res, cobra.ShellCompDirectiveNoFileComp
}

func completePlanets(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var res []string
	if len(toComplete) < 3 {
		log("... enter at least 3 characters")
		return res, cobra.ShellCompDirectiveNoFileComp
	}

	rows, err := db.DB.Query(`
		SELECT designation, life_stage, scanned
		FROM planets
		WHERE starts_with(designation, $1)
	`, strings.ToUpper(toComplete))
	if err != nil {
		log("Can't query prefix %q: %v", toComplete, err)
		return res, cobra.ShellCompDirectiveNoFileComp
	}
	for rows.Next() {
		var d string
		var l, s bool
		if err := rows.Scan(&d, &l, &s); err != nil {
			continue
		}
		res = append(res, fmt.Sprintf("%s\tLife stage: %v, Scanned: %v", d, l, s))
	}

	row := db.DB.QueryRow(`
		SELECT designation, density
		FROM belts
		WHERE starts_with(designation, $1)
	`, toComplete)
	var dg, dn string
	if err := row.Scan(&dg, &dn); err == nil {
		res = append(res, fmt.Sprintf("%s\tDensity: %s", dg, dn))
	}

	return res, cobra.ShellCompDirectiveNoFileComp
}

func completeEventIDs(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var res []string
	data, err := rest.Events()
	if err != nil {
		log("error getting events: %v", err)
		return res, cobra.ShellCompDirectiveError
	}
	ids := make(map[string]string)
	for _, e := range data.Events {
		id := e.Designation
		ids[id] = e.Title
	}

	for id, name := range ids {
		res = append(res, fmt.Sprintf("%s\t%s", id, name))
	}
	return res, cobra.ShellCompDirectiveNoFileComp
}

func completeEventCriteria(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var res []string
	data, err := rest.Events()
	if err != nil {
		log("Error getting events: %v", err)
		return res, cobra.ShellCompDirectiveError
	}
	id := getString(cmd, "id")
	if id == "" {
		return res, cobra.ShellCompDirectiveNoFileComp
	}
	var crits []string
	for _, e := range data.Events {
		if e.Designation != id {
			continue
		}
		for _, c := range e.Criteria {
			crits = append(crits, c.Short())
		}
	}

	if len(crits) == 0 {
		return res, cobra.ShellCompDirectiveError
	}
	for n, c := range crits {
		res = append(res, fmt.Sprintf("%d\t%s", n+1, c))
	}
	return res, cobra.ShellCompDirectiveNoFileComp

}

func completeDevicesFilters(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	keywords := []string{"location", "type", "tag", "destination"}
	res := []string{"--ignore_tags", "--merge=false"}

	if len(args)%2 == 0 {
		return append(res, keywords...), cobra.ShellCompDirectiveNoFileComp
	}
	last := args[len(args)-1]
	switch last {
	case "location", "destination":
		return completeStars(cmd, args, toComplete)
	case "type":
		types, err := db.ListIDs(cache.AliasTypesTable)
		if err != nil {
			return res, cobra.ShellCompDirectiveError
		}
		for _, t := range types {
			res = append(res, t.(string))
		}
	case "tag", "tags":
		q := `
		SELECT distinct JSONB_ARRAY_ELEMENTS_TEXT(data->'tags') AS tags
		FROM json_devices
	  `
		rows, err := db.DB.Query(q)
		if err != nil {
			return res, cobra.ShellCompDirectiveError
		}
		for rows.Next() {
			var t string
			if err := rows.Scan(&t); err == nil {
				res = append(res, t)
			}
		}
	}
	return res, cobra.ShellCompDirectiveNoFileComp
}

func completeDeviceTags(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// find the first argument that gets dealiased
	var code string
	for _, a := range os.Args {
		if dealiased := db.Dealias(a); dealiased != a {
			code = dealiased
			break
		}
	}
	if code == "" {
		return nil, cobra.ShellCompDirectiveError
	}
	rows, err := db.DB.Query(`
    SELECT JSONB_ARRAY_ELEMENTS_TEXT( data->'tags') FROM json_devices WHERE code = $1
  `, code)
	if err != nil {
		log("query err: %v", err)
		return nil, cobra.ShellCompDirectiveError
	}
	var res []string
	for rows.Next() {
		var t string
		rows.Scan(&t)
		res = append(res, t)
	}

	return res, cobra.ShellCompDirectiveNoFileComp
}

var StandardResources = []string{
	"carbon",
	"conductive",
	"rares",
	"silicates",
	"structural",
	"volatiles",
}

func completeResources(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var res []string
	toComplete = strings.ToLower(toComplete)
	for _, r := range StandardResources {
		if strings.HasPrefix(r, toComplete) {
			res = append(res, r)
		}
	}
	return res, cobra.ShellCompDirectiveNoFileComp
}

func completeIntents(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var res []string
	if db == nil || db.DB == nil {
		return res, cobra.ShellCompDirectiveNoFileComp
	}
	rows, err := db.DB.Query("SELECT location FROM intent WHERE starts_with(location, $1) ORDER BY location", strings.ToUpper(toComplete))
	if err != nil {
		return res, cobra.ShellCompDirectiveNoFileComp
	}
	defer rows.Close()
	for rows.Next() {
		var loc string
		if err := rows.Scan(&loc); err == nil {
			res = append(res, loc)
		}
	}
	return res, cobra.ShellCompDirectiveNoFileComp
}

func completeIntentAddArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return completeStarsAndPlanets(cmd, args, toComplete)
	}
	return completeResources(cmd, args, toComplete)
}

func completeIntentRemoveArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return completeIntents(cmd, args, toComplete)
	}
	return completeResources(cmd, args, toComplete)
}
