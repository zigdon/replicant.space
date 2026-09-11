package cache

import (
	"crypto/md5"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

func Encode[T any](in T) JSONB[T] {
	return JSONB[T]{Data: in}
}

type JSONB[T any] struct {
	Data T
}

// Value implements driver.Valuer (Marshals Go struct -> JSONB for INSERT/UPDATE)
func (j JSONB[T]) Value() (driver.Value, error) {
	return json.Marshal(j.Data)
}

// Scan implements sql.Scanner (Unmarshals JSONB -> Go struct for SELECT)
func (j *JSONB[T]) Scan(value any) error {
	if value == nil {
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into JSONB", value)
	}
	return json.Unmarshal(bytes, &j.Data)
}

func PsqlDuration(in string) (time.Duration, error) {
	in = strings.Replace(in, ":", "h", 1)
	in = strings.Replace(in, ":", "m", 1)
	in += "s"
	return time.ParseDuration(in)
}

func (db *Cache) FindNearestStar(x, y, z float32) (string, float32, error) {
	if db == nil || db.DB == nil {
		return "", 0, fmt.Errorf("database cache is not connected")
	}
	row := db.QueryRow(
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

// FindNearestStarInRange finds the nearest star to the destination coordinates (x, y, z)
// that is within maxDist of at least one of the stars in the given list.
func (db *Cache) FindNearestStarInRange(x, y, z float32, stars []string, maxDist float32) (string, float32, error) {
	if db == nil || db.DB == nil {
		return "", 0, fmt.Errorf("database cache is not connected")
	}

	row := db.QueryRow(
		`SELECT s.designation, s.position <-> $1::cube AS dist
		FROM stars s
		WHERE EXISTS (
			SELECT 1
			FROM stars ref
			WHERE ref.designation = ANY($2::text[])
			  AND s.position <-> ref.position <= $3
		)
		ORDER BY dist ASC
		LIMIT 1`,
		Position{x, y, z},
		pq.Array(stars),
		maxDist,
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
		WHERE has_hub
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

func (db *Cache) FindNearestOwnedHub(x, y, z float32) (string, float32, error) {
	row := db.DB.QueryRow(`
		SELECT designation, position <-> $1::cube AS dist
		FROM json_devices JOIN stars ON split_part(location, '-', 1) = designation
		WHERE type = 'system_hub'
		ORDER BY dist ASC
		LIMIT 1`, Position{x, y, z})
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
	Region       string
	Distance     float32
}

func (db *Cache) QueryStarsInRadius(x, y, z, radius float32, limit int) ([]*StarRecord, error) {
	q := `
		SELECT designation, COALESCE(name, ''), entry_point, est_planets,
		       spectral_type, COALESCE(explored, false), COALESCE(has_life, false),
		       position, COALESCE(has_hub, false),
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
			&s.Position, &s.HasHub,
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
		       position, COALESCE(has_hub, false),
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
			&s.Position, &s.HasHub,
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
		       position, COALESCE(has_hub, false),
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
			&s.Position, &s.HasHub,
			&s.Region, &s.Distance,
		); err != nil {
			return nil, err
		}
		stars = append(stars, s)
	}
	return stars, rows.Err()
}

// Opts allows modifying the device query - returning the limits and additional params
type QueryDevicesOpts struct {
	Limits []string
	Params []any
}

func QueryDevicesField(f string, t ...string) QueryDevicesOpts {
	if len(t) == 1 {
		return QueryDevicesOpts{[]string{fmt.Sprintf("%s = $%%d", f)}, []any{t[0]}}
	}
	return QueryDevicesOpts{
		[]string{fmt.Sprintf("%s = ANY($%%d::TEXT[])", f)}, []any{pq.Array(t)},
	}
}

func QueryDevicesStatus(t ...string) QueryDevicesOpts {
	return QueryDevicesField("status", t...)
}

func QueryDevicesType(t ...string) QueryDevicesOpts {
	return QueryDevicesField("type", t...)
}

func QueryDevicesLocation(t ...string) QueryDevicesOpts {
	return QueryDevicesField("location", t...)
}

type QueryDevicesRes struct {
	Code, Type, Location, Status string
	Data                         []byte
}

func (db *Cache) QueryDevices(opts ...QueryDevicesOpts) ([]*QueryDevicesRes, error) {
	if db == nil || db.DB == nil {
		return nil, fmt.Errorf("database cache is not connected")
	}
	var limits []string
	var params []any
	for _, o := range opts {
		limits = append(limits, o.Limits...)
		params = append(params, o.Params...)
	}
	var n = 1
	for i, l := range limits {
		for strings.Contains(l, "%d") {
			l = strings.Replace(l, "%d", fmt.Sprintf("%d", n), 1)
			n++
		}
		limits[i] = l
	}
	log("limits=%v\nparams=%v", limits, params)
	q := fmt.Sprintf(`
		SELECT code, type, location, status, data
		FROM json_devices
		WHERE %s`, strings.Join(limits, " AND "))
	rows, err := db.Query(q, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []*QueryDevicesRes
	for rows.Next() {
		var c, t, l, s string
		var d []byte
		if err := rows.Scan(&c, &t, &l, &s, &d); err != nil {
			return nil, err
		}
		res = append(res, &QueryDevicesRes{
			Code:     c,
			Type:     t,
			Location: l,
			Status:   s,
			Data:     d,
		})
	}

	return res, nil
}

func (db *Cache) ChecksumHubs() (string, error) {
	rows, err := db.Query(`
	  SELECT designation
	  FROM stars
	  WHERE has_hub
	  ORDER BY designation`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	cksum := md5.New()
	for rows.Next() {
		var l []byte
		if err := rows.Scan(&l); err != nil {
			return "", err
		}
		if _, err := cksum.Write(l); err != nil {
			return "", err
		}
	}
	return base64.StdEncoding.EncodeToString(cksum.Sum(nil)), nil
}

type MinedBeltRecord struct {
	Designation string
	Star        string
	Density     string
	Mining      bool
	Resources   map[string]string
}

func (db *Cache) QueryMinedBelts() ([]*MinedBeltRecord, error) {
	if db == nil || db.DB == nil {
		return nil, fmt.Errorf("database cache is not connected")
	}

	q := `
		SELECT designation, COALESCE(NULLIF(star, ''), split_part(designation, '-', 1)), density, COALESCE(mining, false), COALESCE(resources, '{}'::jsonb)
		FROM belts
		WHERE mining = true
		ORDER BY designation ASC`

	rows, err := db.DB.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*MinedBeltRecord
	for rows.Next() {
		r := new(MinedBeltRecord)
		var res JSONB[map[string]string]
		if err := rows.Scan(&r.Designation, &r.Star, &r.Density, &r.Mining, &res); err != nil {
			return nil, err
		}
		r.Star = strings.ToUpper(strings.TrimSpace(r.Star))
		if r.Star == "" {
			parts := strings.Split(r.Designation, "-")
			if len(parts) > 0 {
				r.Star = strings.ToUpper(strings.TrimSpace(parts[0]))
			}
		}
		r.Resources = res.Data
		records = append(records, r)
	}
	return records, rows.Err()
}
