package cache

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	dbPath = "./sql/replicant.db"
	logFile = "/tmp/rsp-query.log"
)

//go:embed schema.sql
var schema string

func log(tmpl string, args ...any) {
	ts := time.Now().Format(time.Stamp)
	line := fmt.Sprintf(ts+" "+tmpl+"\n", args...)
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't open to %q: %v\n", logFile, err)
	} else {
		f.WriteString(line)
		f.Close()
	}
	if os.Getenv("DEBUG_DB") != "" {
		fmt.Fprint(os.Stderr, line)
	}
}

type Tables string
const (
	StarsTable Tables = "stars"
	PlanetsTable Tables = "planets"
	MoonsTable Tables = "moons"
	BeltsTable Tables = "belts"
	ResourcesTable Tables = "resources"
	AliasTable Tables = "aliases"
)

type Cache struct {
	DB *sql.DB
}

func createDB() (*Cache, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	// Create the file if it isn't there.
	sdb, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Create our tables
	_, err = sdb.Exec(schema)
	if err != nil {
		return nil, err
	}

	return &Cache{sdb}, nil
}

func Connect(create bool) (*Cache, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if !create {
			return nil, err
		}
		return createDB()
	}
	sdb, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	return &Cache{sdb}, nil
}

func (db *Cache) UpdateSchema() error {
	_, err := db.DB.Exec(schema)
	return err
}

func (db *Cache) Stats() string {
	var out []string
	for _, t := range []string{"stars", "planets", "moons", "resources", "aliases"} {
		res := db.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) AS c FROM %s", t))
		var cnt int
		err := res.Scan(&cnt)
		if err != nil {
			out = append(out, fmt.Sprintf("%s: %v", t, err))
			continue
		}
		out = append(out, fmt.Sprintf("%s rows: %d", t, cnt))
	}
	return strings.Join(out, "\n")
}

func (db *Cache) Get(table Tables, key string) (any, error) {
	row := db.DB.QueryRow(
		fmt.Sprintf("SELECT * FROM %s WHERE designation = ?", table), key)
	if row.Err() != nil {
		return nil, row.Err()
	}
	switch table {
	case "stars":
		s := &Star{}
		s.Load(row.Scan)
		return s, nil
	}
	return nil, fmt.Errorf("Table %q not found", table)
}

func (db *Cache) Update(table string, data map[string]any) error {
	var columns, placeholders []string
	var values []any
	for k, v := range data {
		columns = append(columns, k)
		values = append(values, v)
		placeholders = append(placeholders, "?")
	}
	q := fmt.Sprintf(
			"REPLACE INTO %s (%s) VALUES (%s)",
			table, strings.Join(columns, ", "),
			strings.Join(placeholders, ", "))
	log("update: %q: %v", q, values)
	res, err := db.DB.Exec(q, values...)

	if err != nil {
		return fmt.Errorf("failed to call REPLACE: %v", err)
	}

	rows, err := res.RowsAffected()
	if rows != 1 || err != nil {
		return fmt.Errorf("%d rows affected: %v", rows, err)
	}

	return nil
}

func (db *Cache) List(table Tables) ([]any, error) {
	rows, err := db.DB.Query(fmt.Sprintf("SELECT * FROM %s", table))
	if err != nil {
		return nil, err
	}
	var res []any
	for rows.Next() {
		switch table {
		case "stars":
			s := &Star{}
			s.Load(rows.Scan)
			res = append(res, s)
		}
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return res, nil
}

// Aliases

var prefixes = map[string]string{
	"maintenance_drone": "mtd",
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
