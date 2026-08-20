package cache

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"database/sql"
	"database/sql/driver"

	_ "github.com/lib/pq" // Register the driver

	"github.com/zigdon/rsp/cfg"
)

//go:embed schema.psql
var schema string

func log(tmpl string, args ...any) {
	ts := time.Now().Format(time.Stamp)
	for n, a := range args {
		if b, ok := a.([]byte); ok {
			s := string(b)
			if len(s) > 10000 {
				args[n] = fmt.Sprintf("[%d]byte: %s...", len(b), s[:10000])
			} else {
				args[n] = fmt.Sprintf("[%d]byte: %s", len(b), s)
			}
		}
	}
	line := fmt.Sprintf(ts+" "+tmpl+"\n", args...)
	if os.Getenv("DEBUG_DB") != "" {
		fmt.Fprint(os.Stderr, line)
	}
}

type Tables string

const (
	AliasTable             Tables = "aliases"
	AliasTypesTable        Tables = "alias_types"
	BeltsTable             Tables = "belts"
	BlueprintCmpTable      Tables = "blueprint_components"
	BlueprintDirsTable     Tables = "blueprint_directives"
	BlueprintFeaturesTable Tables = "blueprint_features"
	BlueprintResTable      Tables = "blueprint_resources"
	BlueprintsTable        Tables = "blueprints"
	DeviceLogsTable        Tables = "device_logs"
	EventsTable            Tables = "event_stream"
	InventoryTable         Tables = "inventory"
	JSONDevices            Tables = "json_devices"
	JourneyStepsTable      Tables = "cached_journey_steps"
	JourneyTable           Tables = "cached_journey"
	MoonsTable             Tables = "moons"
	MsgTable               Tables = "messages"
	NotificationTable      Tables = "notifications"
	PlanetsTable           Tables = "planets"
	StarsTable             Tables = "stars"
)

var cols = map[Tables][]string{
	StarsTable: {
		"designation", "name", "entry_point", "est_planets", "spectral_type",
		"explored", "has_life", "position", "has_hub", "has_my_hub", "region"},
	PlanetsTable: {
		"designation", "star", "name", "life_stage", "moons", "rings", "scanned", "type"},
	MoonsTable: {
		"designation", "planet", "star", "name", "scanned", "type"},
	BeltsTable: {
		"designation", "star", "density", "mining", "resources"},
	AliasTable: {
		"designation", "type", "name"},
	AliasTypesTable: {
		"type", "prefix"},
	BlueprintsTable: {
		"type", "print_time", "attach_capacity", "cargo_capacity", "stow_capacity", "short", "description"},
	BlueprintResTable: {
		"blueprint_type", "type", "qty"},
	BlueprintCmpTable: {
		"blueprint_type", "type", "qty"},
	BlueprintDirsTable: {
		"blueprint_type", "directive"},
	BlueprintFeaturesTable: {
		"blueprint_type", "feature"},
	DeviceLogsTable: {
		"id", "created", "device", "type", "message", "payload"},
	MsgTable: {
		"id", "body", "created", "read", "type", "title"},
	JourneyTable: {
		"id", "origin", "dest", "max_hop", "calculated"},
	JourneyStepsTable: {
		"journey_id", "src", "dest", "dist_src", "dist_dest"},
	JSONDevices: {
		"code", "updated_ts", "location", "data"},
	EventsTable: {
		"id", "category", "created", "code", "event", "location", "data"},
	InventoryTable: {
		"designation", "star", "carbon", "conductive", "rares", "silicates", "structural", "volatiles"},
}

var constraints = map[Tables]string{
	BeltsTable:             "designation",
	BlueprintCmpTable:      "blueprint_type, type",
	BlueprintDirsTable:     "blueprint_type, directive",
	BlueprintFeaturesTable: "blueprint_type, feature",
	BlueprintResTable:      "blueprint_type, type",
	BlueprintsTable:        "type",
	DeviceLogsTable:        "id, device",
	EventsTable:            "id",
	InventoryTable:         "designation",
	JSONDevices:            "code",
	JourneyStepsTable:      "journey_id, step",
	JourneyTable:           "id",
	MoonsTable:             "designation",
	MsgTable:               "id",
	NotificationTable:      "id",
	PlanetsTable:           "designation",
	StarsTable:             "designation",
}

type Cache struct {
	DB *sql.DB
}

func Connect() (*Cache, error) {
	cfg, err := cfg.ReadCfg()
	if err != nil {
		return nil, err
	}
	pdb, err := sql.Open("postgres",
		fmt.Sprintf("host=%s dbname=%s connect_timeout=5 sslmode=prefer", cfg.DBHost, cfg.DBName))

	db := &Cache{pdb}

	// Preload aliases
	rows, err := db.DB.Query(`SELECT type, prefix FROM alias_types`)
	if err != nil {
		log("Couldn't preload aliases: %v", err)
		return db, nil
	}
	prefixes = make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return db, err
		}
		prefixes[k] = v
	}
	return db, rows.Err()
}

func (db *Cache) UpdateSchema() error {
	_, err := db.DB.Exec(schema)
	return err
}

func (db *Cache) Query(query string, args ...any) (*sql.Rows, error) {
	log("%q: %v", query, args)
	return db.DB.Query(query, args...)
}

func (db *Cache) QueryRow(query string, args ...any) *sql.Row {
	log("%q: %v", query, args)
	return db.DB.QueryRow(query, args...)
}

func (db *Cache) Exec(query string, args ...any) (sql.Result, error) {
	log("%q: %v", query, args)
	return db.DB.Exec(query, args...)
}

func (db *Cache) Stats() string {
	var out []string
	for _, t := range []string{"stars", "planets", "moons", "resources", "aliases"} {
		res := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) AS c FROM %s", t))
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
	log("SELECT %s FROM %s WHERE %s = $1", strings.Join(cols[table], ", "), table, cols[table][0])
	row := db.QueryRow(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1",
			strings.Join(cols[table], ", "), table, cols[table][0]), key)
	if row.Err() != nil {
		return nil, row.Err()
	}
	return row.Scan, nil
}

func (db *Cache) GetVal(table Tables, col, key string) (func(...any) error, error) {
	log("SELECT %s FROM %s WHERE %s = $1", col, table, cols[table][0])
	row := db.QueryRow(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1", col, table, cols[table][0]), key)
	if row.Err() != nil {
		return nil, row.Err()
	}
	return row.Scan, nil
}

func (db *Cache) GetAll(table Tables, key string) (*sql.Rows, error) {
	log("SELECT %s FROM %s WHERE %s = $1", strings.Join(cols[table], ", "), table, cols[table][0])
	rows, err := db.Query(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1",
			strings.Join(cols[table], ", "), table, cols[table][0]), key)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (db *Cache) Update(table Tables, data map[string]any) error {
	var columns, placeholders []string
	var values []any
	var updates []string
	n := 1
	for k, v := range data {
		updates = append(updates, fmt.Sprintf("%s=EXCLUDED.%s", k, k))
		columns = append(columns, k)
		values = append(values, v)
		switch v.(type) {
		case Position:
			placeholders = append(placeholders, fmt.Sprintf("$%d::cube", n))
		default:
			placeholders = append(placeholders, fmt.Sprintf("$%d", n))
		}
		n++
	}
	q := fmt.Sprintf(`
		INSERT INTO %s (%s)
		VALUES (%s)
		ON CONFLICT (%s)
		DO UPDATE SET
		%s;
		`,
		table, strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
		constraints[table], strings.Join(updates, ",\n"),
	)

	res, err := db.DB.Exec(q, values...)
	for n, a := range values {
		if b, ok := a.([]byte); ok {
			s := string(b)
			if len(s) > 10000 {
				values[n] = fmt.Sprintf("[%d]byte: %s...", len(b), s[:10000])
			} else {
				values[n] = fmt.Sprintf("[%d]byte: %s", len(b), s)
			}
		}
	}
	log("update: %q: %+v", q, values)

	if err != nil {
		return fmt.Errorf("failed to call REPLACE:\n%s\n%v", q, err)
	}

	rows, err := res.RowsAffected()
	if rows != 1 || err != nil {
		return fmt.Errorf("%d rows affected: %v", rows, err)
	}

	return nil
}

func (db *Cache) Reset(table Tables) error {
	_, err := db.DB.Exec(fmt.Sprintf("DELETE FROM %s", table))
	return err
}

func (db *Cache) ListIDs(table Tables) ([]any, error) {
	log("SELECT %s FROM %s", cols[table][0], table)
	rows, err := db.Query(fmt.Sprintf("SELECT %s FROM %s", cols[table][0], table))
	if err != nil {
		return nil, err
	}
	var res []any
	for rows.Next() {
		var id any
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

func (db *Cache) PendingNotifications(read bool) (*sql.Rows, error) {
	now := time.Now()
	q := fmt.Sprintf(`
		SELECT id, start_ts, end_ts, device, text
		FROM %s
		WHERE read = $1 AND end_ts < $2`, NotificationTable)
	return db.Query(q, read, now)
}

func (db *Cache) ClearNotifications(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	var phs []string
	as := make([]any, len(ids))
	for i := range ids {
		phs = append(phs, fmt.Sprintf("$%d", i+1))
		as[i] = ids[i]
	}
	q := fmt.Sprintf(`
		UPDATE %s
		SET read = true
		WHERE id in (%s)`, NotificationTable, strings.Join(phs, ", "))
	_, err := db.DB.Exec(q, as...)
	return err
}

func (db *Cache) AddDelivery(dryRun bool, origin, destination, carrier string, cargo map[string]int) error {
	data, err := json.Marshal(cargo)
	if err != nil {
		return err
	}
	if dryRun {
		log("[DRYRUN] Adding delivery: %s %s->%s: %v", carrier, origin, destination, cargo)
		return nil
	}
	_, err = db.Exec(`
	  INSERT INTO deliveries (origin, destination, ship, cargo)
	  VALUES ($1, $2, $3, $4)
  `, origin, destination, carrier, data)
	return err
}

func (db *Cache) ClearDelivery(dryRun bool, carrier string) error {
	if dryRun {
		log("[DRYRUN] Clearing delivery: %s", carrier)
		return nil
	}
	_, err := db.Exec(`DELETE FROM deliveries WHERE ship=$1`, carrier)
	return err
}

func (db *Cache) UpdateInventory(dryRun bool, location string, res map[string]int) error {
	if dryRun {
		log("[DRYRUN] Updating inventory at %s: %#v", location, res)
		return nil
	}
	_, err := db.Exec(`
		UPDATE inventory
		SET carbon=carbon+$1,
			conductive=conductive+$2,
			rares=rares+$3,
			silicates=silicates+$4,
			structural=structural+$5,
			volatiles=volatiles+$6
		WHERE designation = $7`,
		res["carbon"], res["conductive"], res["rares"],
		res["silicates"], res["structural"], res["volatiles"],
		location)

	return err
}

func Strs(in []any) []string {
	res := make([]string, len(in))
	for i, v := range in {
		res[i] = v.(string)
	}
	return res
}

func Ints(in []any) []int64 {
	res := make([]int64, len(in))
	for i, v := range in {
		res[i] = v.(int64)
	}
	return res
}

type Position struct {
	X, Y, Z float32
}

// Scan converts the PostgreSQL cube string "(x, y, z)" into a Position.
func (p *Position) Scan(value any) error {
	if value == nil {
		return nil
	}

	var str string
	switch v := value.(type) {
	case []byte:
		str = string(v)
	case string:
		str = v
	default:
		return fmt.Errorf("unsupported type for Position: %T", value)
	}

	_, err := fmt.Sscanf(str, "(%f, %f, %f)", &p.X, &p.Y, &p.Z)
	if err != nil {
		return fmt.Errorf("failed to parse cube string %q: %w", str, err)
	}
	return nil
}

// Value converts a Position into a PostgreSQL-compatible cube string.
func (p Position) Value() (driver.Value, error) {
	return fmt.Sprintf("(%f, %f, %f)", p.X, p.Y, p.Z), nil
}
