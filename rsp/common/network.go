package common

import (
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
	"golang.org/x/sync/errgroup"
)

var _networkCache []string
var _networkCacheTS time.Time
var _ignoreRep *models.CodeAlias

// Get all the systems currently in range
func FullNetwork(ignoreRep *models.CodeAlias) ([]string, error) {
	if _networkCache != nil && time.Since(_networkCacheTS) < 10*time.Minute && ignoreRep == _ignoreRep {
		return _networkCache, nil
	}

	// Loop over all the replicants
	// In each stationary one, add that system
	// Find relay devices in the system, add their connections
	net := make(map[string]bool)
	acc, err := rest.Account()
	if err != nil {
		return nil, err
	}
	var eg errgroup.Group
	eg.SetLimit(5)
	var mu sync.Mutex
	for _, r := range acc.ReplicantList {
		if r.CurrentLocation == "" || r.Code == ignoreRep {
			continue
		}
		mu.Lock()
		net[r.Location.Star()] = true
		mu.Unlock()
		eg.Go(func() error {
			relay := db.QueryRow(`
				SELECT DISTINCT ON (code) code
				FROM json_devices
				WHERE type IN ('ftl_relay', 'deep_space_relay_station', 'system_hub')
				  AND status = 'relaying'
				  AND location = $1
				GROUP BY location, type, code`, r.Location)
			var dc string
			if err := relay.Scan(&dc); err != nil {
				if strings.Contains(err.Error(), "no rows in result set") {
					return nil
				}
				return err
			}
			ca := models.NewCodeAlias(dc)
			// Log("Found relay at %q: %s", r.Location, ca.Alias())
			con, err := rest.DeviceNetwork(ca)
			if err != nil {
				return err
			}
			var added int
			mu.Lock()
			defer mu.Unlock()
			for _, node := range con.Connections {
				if net[node.Star] {
					continue
				}
				net[node.Star] = true
				added++
			}
			// if added > 0 {
			// 	Log("Added %d new nodes", added)
			// }
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	Log("Found %d systems in the network", len(net))
	var res []string
	for k := range net {
		res = append(res, k)
	}
	slices.Sort(res)

	_networkCacheTS = time.Now()
	_networkCache = res
	_ignoreRep = ignoreRep
	return res, err
}
