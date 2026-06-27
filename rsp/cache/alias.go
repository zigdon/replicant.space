package cache

import (
	"fmt"
	"strings"
)

// Aliases

var prefixes = map[string]string{
	"maintenance_drone": "mtd",
	"surge_platform": "spf",
}

func (db *Cache) GetAliasAndType(code string) (string, string) {
	row := db.DB.QueryRow("SELECT name, type FROM aliases WHERE designation = ?", code)
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
	row := db.DB.QueryRow("SELECT designation FROM aliases WHERE name = ?", alias)
	var code string
	if err := row.Scan(&code); err != nil {
		return alias
	}
	return code
}

func (db *Cache) HasAlias(designation string) string {
	row := db.DB.QueryRow("SELECT name FROM aliases WHERE designation = ?", designation)
	if row.Err() != nil {
		return ""
	}
	var alias string
	if err := row.Scan(&alias); err == nil {
		return alias
	}
	return ""
}

func (db *Cache) Alias(designation, deviceType string) (string, error) {
	// This is already an alias, return unchanged
	if strings.Contains(designation, "-") || designation == "" {
		return designation, nil
	}

	// See if there's already an alias
	row := db.DB.QueryRow("SELECT name FROM aliases WHERE designation = ?", designation)
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
	prefix, ok := prefixes[deviceType]
	if !ok {
		// No preset, make one up
		for w := range strings.SplitSeq(deviceType, "_") {
			prefix = fmt.Sprintf("%s%c", prefix, w[0])
		}
	}

	// Find how many of these prefixes we already have
	row = db.DB.QueryRow("SELECT COUNT(*) FROM aliases WHERE type = ?", deviceType)
	var cnt int
	if err := row.Scan(&cnt); err != nil {
		return "", err
	}

	// Save the new prefix
	alias = fmt.Sprintf("%s-%d", prefix, cnt+1)
	log("Adding new alias %q -> %q", designation, alias)
	if _, err := db.DB.Exec("INSERT INTO aliases (designation, type, name) VALUES (?, ?, ?)",
		designation, deviceType, alias); err != nil {
			return "", err
	}
	return alias, nil
}
