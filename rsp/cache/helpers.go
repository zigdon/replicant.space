package cache

import (
	"math"
	"slices"
)

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

func (db *Cache) DeviceLogCursor(devID string) int {
	row := db.DB.QueryRow(`SELECT max(id) FROM device_logs WHERE device = $1`, devID)
	var id int
	if err := row.Scan(&id); err != nil {
		log("Error getting cursor for %q: %v", devID, err)
	}
	return id
}
