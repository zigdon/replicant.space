package cache

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zigdon/rsp/models"
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
	if os.Getenv("DEBUG_API") != "" {
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
)

type db struct {
	DB *sql.DB
}

func createDB() (*db, error) {
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

	return &db{sdb}, nil
}

func Connect(create bool) (*db, error) {
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
	return &db{sdb}, nil
}

func (db *db) Stats() string {
	var out []string
	for _, t := range []string{"stars", "planets", "moons", "resources"} {
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

func (db *db) Get(table Tables, key string) (any, error) {
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

func (db *db) Update(table string, data map[string]any) error {
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

func (db *db) List(table Tables) ([]any, error) {
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

func (db *db) TripStepCandidate(srcStar, dstStar string, radius float32) ([]*models.JourneyLeg, error) {
	// Get source coords
	entry, err := db.Get(StarsTable, srcStar)
	if err != nil {
		return nil, err
	}
	src := entry.(*Star)
	// Get dest coords
	entry, err = db.Get(StarsTable, dstStar)
	if err != nil {
		return nil, err
	}
	dst := entry.(*Star)
	rows, err := db.DB.Query(
		`SELECT designation,
			position_x,
			position_y,
			position_z,
		    sqrt(
				power(position_x-?,2) +
				power(position_y-?,2) +
				power(position_z-?,2)) AS from_src,
		    sqrt(
				power(position_x-?,2) +
				power(position_y-?,2) +
				power(position_z-?,2)) AS from_dst
		FROM stars WHERE from_src <= ? AND from_src > 0.001`,
		src.PositionX,
		src.PositionY,
		src.PositionZ,
		dst.PositionX,
		dst.PositionY,
		dst.PositionZ,
		radius,
	)
	if err != nil {
		return nil, err
	}

	var res []*models.JourneyLeg
	for rows.Next() {
		var desg string
		var x, y, z, fSrc, fDst float32
		rows.Scan(&desg, &x, &y, &z, &fSrc, &fDst)
		res = append(res, &models.JourneyLeg{
				From: src.Designation,
				FromPosition: models.NewPosition(
					src.PositionX,
					src.PositionY,
					src.PositionZ),
				To: desg,
				ToPosition: models.NewPosition(x, y, z),
				DistFromSrc: fSrc,
				DistToDest: fDst,
			},
		)
	}

	return res, nil
}

