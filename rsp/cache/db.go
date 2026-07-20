package cache

import (
	"database/sql"
	_ "embed"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"time"

	_ "github.com/lib/pq" // Register the driver

	"github.com/zigdon/rsp/cfg"
)

const (
	logFile = "/tmp/rsp-query.log"
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
	StarsTable             Tables = "stars"
	PlanetsTable           Tables = "planets"
	MoonsTable             Tables = "moons"
	BeltsTable             Tables = "belts"
	BeltResTable           Tables = "belt_resources"
	AliasTable             Tables = "aliases"
	AliasTypesTable        Tables = "alias_types"
	BlueprintsTable        Tables = "blueprints"
	BlueprintResTable      Tables = "blueprint_resources"
	BlueprintCmpTable      Tables = "blueprint_components"
	BlueprintDirsTable     Tables = "blueprint_directives"
	BlueprintFeaturesTable Tables = "blueprint_features"
	NotificationTable      Tables = "notifications"
	MsgTable               Tables = "messages"
	JourneyTable           Tables = "cached_journey"
	JourneyStepsTable      Tables = "cached_journey_steps"
	JSONDevices            Tables = "json_devices"
)

var cols = map[Tables][]string{
	StarsTable: {
		"designation", "name", "entry_point", "est_planets", "spectral_type",
		"explored", "has_life", "position_x", "position_y", "position_z"},
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
	MsgTable: {
		"id", "body", "created", "read", "type", "title"},
	JourneyTable: {
		"id", "origin", "dest", "max_hop", "calculated"},
	JourneyStepsTable: {
		"journey_id", "src", "dest", "dist_src", "dist_dest"},
	JSONDevices: {
		"code", "updated_ts", "location", "data"},
}

var constraints = map[Tables]string{
	BeltResTable:           "belt, resource",
	BeltsTable:             "designation",
	BlueprintCmpTable:      "blueprint_type, type",
	BlueprintDirsTable:     "blueprint_type, directive",
	BlueprintFeaturesTable: "blueprint_type, feature",
	BlueprintResTable:      "blueprint_type, type",
	BlueprintsTable:        "type",
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
	log("SELECT %s FROM %s WHERE %s = $1", strings.Join(cols[table], ", "), table, cols[table][0])
	row := db.DB.QueryRow(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1",
			strings.Join(cols[table], ", "), table, cols[table][0]), key)
	if row.Err() != nil {
		return nil, row.Err()
	}
	return row.Scan, nil
}

func (db *Cache) GetVal(table Tables, col, key string) (func(...any) error, error) {
	log("SELECT %s FROM %s WHERE %s = $1", col, table, cols[table][0])
	row := db.DB.QueryRow(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1", col, table, cols[table][0]), key)
	if row.Err() != nil {
		return nil, row.Err()
	}
	return row.Scan, nil
}

func (db *Cache) GetAll(table Tables, key string) (*sql.Rows, error) {
	log("SELECT %s FROM %s WHERE %s = $1", strings.Join(cols[table], ", "), table, cols[table][0])
	rows, err := db.DB.Query(
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
		placeholders = append(placeholders, fmt.Sprintf("$%d", n))
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
		return fmt.Errorf("failed to call REPLACE: %v", err)
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
	rows, err := db.DB.Query(fmt.Sprintf("SELECT %s FROM %s", cols[table][0], table))
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
	return db.DB.Query(q, read, now)
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

func (db *Cache) FindNearestStar(x, y, z float32) (string, float32, error) {
	row := db.DB.QueryRow(
		`SELECT designation, position_x, position_y, position_z,
		    sqrt(
				power(position_x-$1,2) +
				power(position_y-$2,2) +
				power(position_z-$3,2)) AS dist
		FROM stars ORDER BY dist ASC LIMIT 1`,
		x, y, z,
	)
	if row.Err() != nil {
		return "", 0, row.Err()
	}
	var dsg string
	var dist float32
	err := row.Scan(
		&dsg, &x, &y, &z, &dist,
	)
	return dsg, dist, err
}

func (db *Cache) FindNearestHub(x, y, z float32) (string, float32, error) {
	row := db.DB.QueryRow(
		`SELECT designation, position_x, position_y, position_z,
		    sqrt(
				power(position_x-$1,2) +
				power(position_y-$2,2) +
				power(position_z-$3,2)) AS dist
		FROM stars
		WHERE has_my_hub
		ORDER BY dist ASC LIMIT 1`,
		x, y, z,
	)
	if row.Err() != nil {
		return "", 0, row.Err()
	}
	var dsg string
	var dist float32
	err := row.Scan(
		&dsg, &x, &y, &z, &dist,
	)
	return dsg, dist, err
}

func (db *Cache) GetSector(x, y, z float32, cone, margin int) ([]string, error) {
	dist := float32(math.Sqrt(float64(x*x + y*y + z*z)))
	log("dist=%v, margin=%v", dist, margin)
	minLY := dist * float32(100-margin) / 100
	maxLY := dist * float32(100+margin) / 100
	deg := float32(cone) / 100
	rows, err := db.DB.Query(`
		SELECT designation,
		    sqrt(
				power(position_x,2) +
				power(position_y,2) +
				power(position_z,2)) AS dist_src
		FROM stars
		WHERE dist_src BETWEEN $1 AND $2 AND
			position_x BETWEEN $3 AND $4 AND
			position_y BETWEEN $5 AND $6 AND
			position_z BETWEEN $7 AND $8`,
		minLY, maxLY,
		slices.Min([]float32{x * (1 - deg), x * (1 + deg)}),
		slices.Max([]float32{x * (1 - deg), x * (1 + deg)}),
		slices.Min([]float32{y * (1 - deg), y * (1 + deg)}),
		slices.Max([]float32{y * (1 - deg), y * (1 + deg)}),
		slices.Min([]float32{z * (1 - deg), z * (1 + deg)}),
		slices.Max([]float32{z * (1 - deg), z * (1 + deg)}),
	)
	if err != nil {
		return []string{}, err
	}
	var res []string
	for rows.Next() {
		var dsg string
		var f float32
		if err := rows.Scan(&dsg, &f); err != nil {
			return res, err
		}
		res = append(res, dsg)
	}

	return res, rows.Err()
}

func (db *Cache) ExpireCache(keep map[string]bool) (int64, error) {
	res, err := db.DB.Exec(`
		DELETE from json_devices
		WHERE updated_ts < NOW() - INTERVAL '5 minutes';
	`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
