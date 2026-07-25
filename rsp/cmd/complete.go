package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
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
		log("... enter at least 3 characters")
		return res, cobra.ShellCompDirectiveNoFileComp
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
	var errs []error
	for rows.Next() {
		var d, ep string
		var e, l bool
		if err := rows.Scan(&d, &e, &l, &ep); err != nil {
			errs = append(errs, err)
			continue
		}
		res = append(res, fmt.Sprintf("%s\tExplored: %v, Has life: %v", d, e, l))
		res = append(res, fmt.Sprintf("%s\tEntry point", ep))
	}
	errs = append(errs, rows.Err())

	if err := errors.Join(errs...); err != nil {
		log("Query err: %v", err)
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
	var errs []error
	for rows.Next() {
		var d string
		var l, s bool
		if err := rows.Scan(&d, &l, &s); err != nil {
			errs = append(errs, err)
			continue
		}
		res = append(res, fmt.Sprintf("%s\tLife stage: %v, Scanned: %v", d, l, s))
	}
	errs = append(errs, rows.Err())

	row := db.DB.QueryRow(`
		SELECT designation, density
		FROM belts
		WHERE starts_with(designation, $1)
	`, toComplete)
	var dg, dn string
	if err := row.Scan(&dg, &dn); err != nil {
		errs = append(errs, err)
	} else {
		res = append(res, fmt.Sprintf("%s\tDensity: %s", dg, dn))
	}

	if err := errors.Join(errs...); err != nil {
		log("Query err: %v", err)
	}

	return res, cobra.ShellCompDirectiveNoFileComp
}
