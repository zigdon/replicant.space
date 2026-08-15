package cache

import (
	"database/sql"
	"fmt"
	"strings"
)

func (db *Cache) FindNearestStar(x, y, z float32) (string, float32, error) {
	row := db.DB.QueryRow(
		`SELECT designation, position <-> $1::cube AS dist
		FROM stars ORDER BY dist ASC LIMIT 1`,
		Position{x, y, z},
	)
	if row.Err() != nil {
		return "", 0, row.Err()
	}
	var dsg string
	var dist float32
	err := row.Scan(
		&dsg, &dist,
	)
	return dsg, dist, err
}

func (db *Cache) FindNearestHub(x, y, z float32) (string, float32, error) {
	row := db.DB.QueryRow(
		`SELECT designation, position <-> $1::cube AS dist
		FROM stars
		WHERE has_my_hub
		ORDER BY dist ASC LIMIT 1`,
		Position{x, y, z},
	)
	if row.Err() != nil {
		return "", 0, row.Err()
	}
	var dsg string
	var dist float32
	err := row.Scan(
		&dsg, &dist,
	)
	return dsg, dist, err
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

func (db *Cache) DeviceLogCursor(devID string) int {
	row := db.DB.QueryRow(`SELECT max(id) FROM device_logs WHERE device = $1`, devID)
	var id int
	if err := row.Scan(&id); err != nil {
		log("Error getting cursor for %q: %v", devID, err)
	}
	return id
}

type StarRecord struct {
	Designation  string
	Name         string
	EntryPoint   string
	EstPlanets   int
	SpectralType string
	Explored     bool
	HasLife      bool
	Position     Position
	HasHub       bool
	HasMyHub     bool
	Region       string
	Distance     float32
}

func (db *Cache) QueryStarsInRadius(x, y, z, radius float32, limit int) ([]*StarRecord, error) {
	q := `
		SELECT designation, COALESCE(name, ''), entry_point, est_planets,
		       spectral_type, COALESCE(explored, false), COALESCE(has_life, false),
		       position, COALESCE(has_hub, false), COALESCE(has_my_hub, false),
		       region, position <-> $1::cube AS dist
		FROM stars
		WHERE position <-> $1::cube <= $2
		ORDER BY dist ASC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.DB.Query(q, Position{x, y, z}, radius)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stars []*StarRecord
	for rows.Next() {
		s := new(StarRecord)
		if err := rows.Scan(
			&s.Designation, &s.Name, &s.EntryPoint, &s.EstPlanets,
			&s.SpectralType, &s.Explored, &s.HasLife,
			&s.Position, &s.HasHub, &s.HasMyHub,
			&s.Region, &s.Distance,
		); err != nil {
			return nil, err
		}
		stars = append(stars, s)
	}
	return stars, rows.Err()
}

func (db *Cache) QueryStarsInBox(minX, minY, minZ, maxX, maxY, maxZ float32, limit int) ([]*StarRecord, error) {
	q := `
		SELECT designation, COALESCE(name, ''), entry_point, est_planets,
		       spectral_type, COALESCE(explored, false), COALESCE(has_life, false),
		       position, COALESCE(has_hub, false), COALESCE(has_my_hub, false),
		       region, 0.0 AS dist
		FROM stars
		WHERE (position->1 BETWEEN $1 AND $4)
		  AND (position->2 BETWEEN $2 AND $5)
		  AND (position->3 BETWEEN $3 AND $6)`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.DB.Query(q, minX, minY, minZ, maxX, maxY, maxZ)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stars []*StarRecord
	for rows.Next() {
		s := new(StarRecord)
		if err := rows.Scan(
			&s.Designation, &s.Name, &s.EntryPoint, &s.EstPlanets,
			&s.SpectralType, &s.Explored, &s.HasLife,
			&s.Position, &s.HasHub, &s.HasMyHub,
			&s.Region, &s.Distance,
		); err != nil {
			return nil, err
		}
		stars = append(stars, s)
	}
	return stars, rows.Err()
}

func (db *Cache) QueryAllStars(limit int) ([]*StarRecord, error) {
	q := `
		SELECT designation, COALESCE(name, ''), entry_point, est_planets,
		       spectral_type, COALESCE(explored, false), COALESCE(has_life, false),
		       position, COALESCE(has_hub, false), COALESCE(has_my_hub, false),
		       region, 0.0 AS dist
		FROM stars`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.DB.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stars []*StarRecord
	for rows.Next() {
		s := new(StarRecord)
		if err := rows.Scan(
			&s.Designation, &s.Name, &s.EntryPoint, &s.EstPlanets,
			&s.SpectralType, &s.Explored, &s.HasLife,
			&s.Position, &s.HasHub, &s.HasMyHub,
			&s.Region, &s.Distance,
		); err != nil {
			return nil, err
		}
		stars = append(stars, s)
	}
	return stars, rows.Err()
}

type DeviceRecord struct {
	Code     string
	Type     string
	Location string
	Status   string
}

func (db *Cache) QueryDevicesByTypes(types []string) ([]*DeviceRecord, error) {
	if db == nil || db.DB == nil {
		return nil, fmt.Errorf("database cache is not connected")
	}

	var filterTypes []string
	matchAll := false
	for _, t := range types {
		trimmed := strings.ToLower(strings.TrimSpace(t))
		if trimmed == "*" || trimmed == "all" {
			matchAll = true
			break
		}
		if trimmed != "" {
			filterTypes = append(filterTypes, trimmed)
		}
	}

	var rows *sql.Rows
	var err error

	if matchAll || len(filterTypes) == 0 {
		q := `
			SELECT code, type, location, COALESCE(status, '')
			FROM json_devices
			WHERE location IS NOT NULL AND location != ''
			ORDER BY type ASC, code ASC`
		rows, err = db.DB.Query(q)
	} else {
		placeholders := make([]string, len(filterTypes))
		vals := make([]any, len(filterTypes))
		for i, t := range filterTypes {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			vals[i] = t
		}
		q := fmt.Sprintf(`
			SELECT code, type, location, COALESCE(status, '')
			FROM json_devices
			WHERE LOWER(type) IN (%s) AND location IS NOT NULL AND location != ''
			ORDER BY type ASC, code ASC`, strings.Join(placeholders, ", "))
		rows, err = db.DB.Query(q, vals...)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devs []*DeviceRecord
	for rows.Next() {
		d := new(DeviceRecord)
		if err := rows.Scan(&d.Code, &d.Type, &d.Location, &d.Status); err != nil {
			return nil, err
		}
		devs = append(devs, d)
	}
	return devs, rows.Err()
}
