package cache

import (
	"fmt"
	"strings"
)

// Aliases
var prefixes = map[string]string{
	"maintenance_drone": "mtd",
	"mass_driver":       "mdr",
	"service_bot":       "svb",
	"surge_platform":    "spf",
}

func (db *Cache) GetAliasAndType(code string) (string, string) {
	row := db.DB.QueryRow("SELECT name, type FROM aliases WHERE designation = $1", code)
	if row.Err() != nil {
		return "", ""
	}
	var alias, deviceType string
	if err := row.Scan(&alias, &deviceType); err == nil {
		return alias, deviceType
	}
	return "", ""
}

func (db *Cache) Dealias(alias string) string {
	// If it's not an alias, nothing to do here
	if !strings.Contains(alias, "-") {
		return alias
	}

	// Look it up
	row := db.DB.QueryRow("SELECT designation FROM aliases WHERE name = $1", alias)
	var code string
	if err := row.Scan(&code); err != nil {
		return alias
	}
	return code
}

func (db *Cache) HasAlias(designation string) string {
	row := db.DB.QueryRow("SELECT name FROM aliases WHERE designation = $1", designation)
	if row.Err() != nil {
		return ""
	}
	var alias string
	if err := row.Scan(&alias); err == nil {
		return alias
	}
	return ""
}

func (db *Cache) GetPrefixForType(t string) string {
	row := db.DB.QueryRow(`SELECT prefix FROM alias_types WHERE type = $1`, t)
	var a string
	if err := row.Scan(&a); err != nil {
		log("%v", err)
	}
	return a
}

func (db *Cache) GetTypeForPrefix(a string) string {
	row := db.DB.QueryRow(`SELECT type FROM alias_types WHERE prefix = $1`, a)
	var t string
	if err := row.Scan(&t); err != nil {
		log("%v", err)
	}
	return t
}

func (db *Cache) AddAliasType(prefix, t string) error {
	_, err := db.DB.Exec(
		"INSERT INTO alias_types (type, prefix) VALUES ($1, $2)",
		t, prefix)
	return err
}

func (db *Cache) AliasType(t string) (string, error) {
	log("Getting prefix for %q", t)
	prefix, ok := prefixes[t]
	if !ok {
		log("Creating a prefix for %q", t)
		// No preset, make one up
		for w := range strings.SplitSeq(t, "_") {
			prefix = fmt.Sprintf("%s%c", prefix, w[0])
		}
		// Check it's not a dup
		for k, v := range prefixes {
			if v == prefix {
				return t, fmt.Errorf("Prefix conflict for new device type %q: %q is already %q", t, prefix, k)
			}
		}
		prefixes[t] = prefix
		if err := db.AddAliasType(prefix, t); err != nil {
			log("Error inserting new alias prefix %q:%q: %v", t, prefix)
		}
	}
	return prefix, nil
}

func (db *Cache) Alias(designation, deviceType string) (string, error) {
	// This is already an alias, return unchanged
	if strings.Contains(designation, "-") || designation == "" {
		return designation, nil
	}

	// See if there's already an alias
	row := db.DB.QueryRow("SELECT name FROM aliases WHERE designation = $1", designation)
	if row.Err() != nil {
		log("Error getting alias for %q: %v", designation, row.Err())
		return "", row.Err()
	}
	var alias string
	err := row.Scan(&alias)
	if err == nil {
		return alias, nil
	}

	// If we don't know the device type, can't make a new alias, so just return the original
	if deviceType == "" {
		return designation, nil
	}

	// No alias found, figure out the prefix
	// If we have one preset, use that
	prefix, err := db.AliasType(deviceType)
	if err != nil {
		return prefix, err
	}

	// Find how many of these prefixes we already have
	row = db.DB.QueryRow("SELECT COUNT(*) FROM aliases WHERE type = $1", deviceType)
	var cnt int
	if err := row.Scan(&cnt); err != nil {
		return "", err
	}

	// Save the new prefix
	alias = fmt.Sprintf("%s-%d", prefix, cnt+1)
	log("Adding new alias %q (%q) -> %q", designation, deviceType, alias)
	if _, err := db.DB.Exec(
		"INSERT INTO aliases (designation, type, name) VALUES ($1, $2, $3)",
		designation, deviceType, alias); err != nil {
		return "", err
	}
	return alias, nil
}
