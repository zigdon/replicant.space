package common

import (
	"slices"
	"strings"
	"time"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
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
	for _, r := range acc.ReplicantList {
		if r.CurrentLocation == "" || r.Code == ignoreRep {
			continue
		}
		net[r.Location.Star()] = true
		relay := db.QueryRow(`
		SELECT code
		FROM json_devices
		WHERE type IN ('ftl_relay', 'deep_space_relay_station', 'system_hub')
		  AND location = $1`, r.Location)
		var dc string
		if err := relay.Scan(&dc); err != nil {
			if strings.Contains(err.Error(), "no rows in result set") {
				continue
			}
			return nil, err
		}
		ca := models.NewCodeAlias(dc)
		Log("Found relay at %q: %s", r.Location, ca.Alias())
		con, err := rest.DeviceNetwork(ca)
		if err != nil {
			return nil, err
		}
		var added int
		for _, node := range con.Connections {
			if net[node.Star] {
				continue
			}
			net[node.Star] = true
			added++
		}
		if added > 0 {
			Log("Added %d new nodes", added)
		}
	}
	Log("Found %d systems in the network", len(net))
	var res []string
	for k := range net {
		res = append(res, k)
	}
	slices.Sort(res)

	_networkCacheTS = time.Now()
	_networkCache = res
	return res, err
}
