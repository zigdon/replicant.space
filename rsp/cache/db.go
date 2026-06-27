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
	BeltResTable Tables = "belt_resources"
	AliasTable Tables = "aliases"
	BlueprintsTable Tables = "blueprints"
	BlueprintResTable Tables = "blueprints_resources"
	BlueprintDirsTable Tables = "blueprints_directives"
	BlueprintFeaturesTable Tables = "blueprints_features"
)

var cols = map[Tables][]string{
	StarsTable: {
		"designation", "name", "entry_point", "est_planets", "explored", "has_life",
		"position_x", "position_y", "position_z"},
	PlanetsTable: {
		"designation", "star", "name", "life_stage", "moons", "rings", "scanned", "type"},
	MoonsTable: {
		"designation", "planet", "star", "name", "scanned", "type"},
	BeltsTable: {
		"designation", "star", "density"},
	BeltResTable: {
		"belt", "resource", "density"},
	AliasTable: {
		"designation", "type", "name"},
	BlueprintsTable: {
		"type", "print_time", "attach_capacity", "cargo_capacity", "stow_capacity"},
	BlueprintResTable: {
		"blueprint_type", "type", "qty"},
	BlueprintDirsTable: {
		"blueprint_type", "directive"},
	BlueprintFeaturesTable: {
		"blueprint_type", "feature"},
}

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

func (db *Cache) Get(table Tables, key string) (func(...any) error, error) {
	row := db.DB.QueryRow(
		fmt.Sprintf("SELECT %s FROM %s WHERE designation = ?",
			strings.Join(cols[table], ", "), table), key)
	if row.Err() != nil {
		return nil, row.Err()
	}
	return row.Scan, nil
}

func (db *Cache) Update(table Tables, data map[string]any) error {
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

func (db *Cache) ListIDs(table Tables) ([]string, error) {
	rows, err := db.DB.Query(fmt.Sprintf("SELECT %s FROM %s", cols[table][0], table))
	if err != nil {
		return nil, err
	}
	var res []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		res = append(res, id)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return res, nil
}

