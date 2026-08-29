package common

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
)

var rockTS time.Time
var rocks []*models.Object

func GetRocks() ([]*models.Object, error) {
	if db == nil {
		return nil, fmt.Errorf("database not connected")
	}
	rows, err := db.GetAll(cache.ObjectsTable)
	if err != nil {
		return nil, err
	}
	var objs []*models.Object
	stat := make(map[string]int)
	for rows.Next() {
		o := &models.Object{}
		var star, source any
		var t time.Time
		if err := rows.Scan(
			&o.Designation, &star, &o.Status, &source, &o.ImpactTarget,
			&o.SizeClass, &t, &o.RequiredStrength); err != nil {
			return nil, err
		}
		o.ImpactEta = models.NewJsonTime(t)
		stat[o.Status]++
		objs = append(objs, o)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	fmt.Printf(" %d rocks: %v\n", len(objs), stat)

	slices.SortFunc(objs, func(a, b *models.Object) int {
		return cmp.Compare(a.Designation.Star(), b.Designation.Star())
	})

	rocks = objs
	rockTS = time.Now()
	return objs, nil
}
