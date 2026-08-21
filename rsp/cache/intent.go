package cache

import (
	"fmt"
	"strings"
)

type IntentRecord struct {
	ID        int
	Location  string
	Demand    map[string]int
	Inventory map[string]int
}

func (db *Cache) GetIntents(location string) ([]*IntentRecord, error) {
	if db == nil || db.DB == nil {
		return nil, fmt.Errorf("database cache is not connected")
	}

	var q string
	var args []any

	if location != "" {
		q = `
			SELECT i.id, i.location, i.demand,
			       COALESCE(inv.carbon, 0), COALESCE(inv.conductive, 0),
			       COALESCE(inv.rares, 0), COALESCE(inv.silicates, 0),
			       COALESCE(inv.structural, 0), COALESCE(inv.volatiles, 0)
			FROM intent i
			LEFT JOIN inventory inv ON i.location = inv.designation
			WHERE i.location = $1
			ORDER BY i.location ASC`
		args = append(args, location)
	} else {
		q = `
			SELECT i.id, i.location, i.demand,
			       COALESCE(inv.carbon, 0), COALESCE(inv.conductive, 0),
			       COALESCE(inv.rares, 0), COALESCE(inv.silicates, 0),
			       COALESCE(inv.structural, 0), COALESCE(inv.volatiles, 0)
			FROM intent i
			LEFT JOIN inventory inv ON i.location = inv.designation
			ORDER BY i.location ASC`
	}

	rows, err := db.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*IntentRecord
	for rows.Next() {
		var id int
		var loc string
		var demandBytes JSONB[map[string]int]
		var ca, co, ra, si, st, vo int
		if err := rows.Scan(&id, &loc, &demandBytes, &ca, &co, &ra, &si, &st, &vo); err != nil {
			return nil, err
		}

		demand := demandBytes.Data

		inv := map[string]int{
			"carbon":     ca,
			"conductive": co,
			"rares":      ra,
			"silicates":  si,
			"structural": st,
			"volatiles":  vo,
		}

		records = append(records, &IntentRecord{
			ID:        id,
			Location:  loc,
			Demand:    demand,
			Inventory: inv,
		})
	}

	return records, rows.Err()
}

func (db *Cache) AddIntent(location string, resources map[string]int) error {
	if db == nil || db.DB == nil {
		return fmt.Errorf("database cache is not connected")
	}
	if location == "" {
		return fmt.Errorf("location cannot be empty")
	}
	if len(resources) == 0 {
		return fmt.Errorf("resources cannot be empty")
	}

	// Fetch existing demand if any
	row := db.DB.QueryRow("SELECT demand FROM intent WHERE location = $1", location)
	var demandBytes JSONB[map[string]int]
	if err := row.Scan(&demandBytes); err != nil {
		return err
	}
	existingDemand := demandBytes.Data

	for k, v := range resources {
		cleanK := strings.ToLower(strings.TrimSpace(k))
		if v > 0 {
			existingDemand[cleanK] = v
		} else {
			delete(existingDemand, cleanK)
		}
	}

	if len(existingDemand) == 0 {
		_, err := db.DB.Exec("DELETE FROM intent WHERE location = $1", location)
		return err
	}

	_, err := db.DB.Exec(`
		INSERT INTO intent (location, demand)
		VALUES ($1, $2)
		ON CONFLICT (location)
		DO UPDATE SET demand = EXCLUDED.demand
	`, location, Encode(existingDemand))

	return err
}

func (db *Cache) RemoveIntent(location string, resources []string) error {
	if db == nil || db.DB == nil {
		return fmt.Errorf("database cache is not connected")
	}
	if location == "" {
		return fmt.Errorf("location cannot be empty")
	}

	if len(resources) == 0 {
		res, err := db.DB.Exec("DELETE FROM intent WHERE location = $1", location)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return fmt.Errorf("no intent found for %s", location)
		}
		return nil
	}

	row := db.DB.QueryRow("SELECT demand FROM intent WHERE location = $1", location)
	var demandBytes JSONB[map[string]int]
	if err := row.Scan(&demandBytes); err != nil {
		return fmt.Errorf("no intent found for %s: %w", location, err)
	}

	existingDemand := demandBytes.Data
	for _, r := range resources {
		delete(existingDemand, strings.ToLower(strings.TrimSpace(r)))
	}

	if len(existingDemand) == 0 {
		_, err := db.DB.Exec("DELETE FROM intent WHERE location = $1", location)
		return err
	}

	_, err := db.DB.Exec("UPDATE intent SET demand = $1 WHERE location = $2", Encode(existingDemand), location)
	return err
}
