package cache

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
