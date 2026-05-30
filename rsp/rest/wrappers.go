package rest

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/zigdon/rsp/models"
)

type cacheEntry struct {
	ts time.Time
	res []byte
}

var cachedCalls map[string]cacheEntry

func cachePOST(key string, ttl time.Duration, path string, data []byte, args ...any) ([]byte, error) {
	if cachedCalls == nil {
		cachedCalls = make(map[string]cacheEntry)
	}
	if ttl == 0 { ttl = time.Minute }
	if key == "" {
		key = fmt.Sprintf("%s:%v:%v", path, args, string(data))
	}
	now := time.Now()
	ent, ok := cachedCalls[key]
	if ok && now.Sub(ent.ts) <= ttl {
		return ent.res, nil
	}
	res, err := Post(path, data, args...)
	if err != nil {
		return nil, err
	}
	cachedCalls[key] = cacheEntry{
		ts: now,
		res: res,
	}
	return res, nil
}

func cacheGET(key string, ttl time.Duration, path string, args ...any) ([]byte, error) {
	if cachedCalls == nil {
		cachedCalls = make(map[string]cacheEntry)
	}
	if ttl == 0 { ttl = time.Minute }
	if key == "" {
		key = fmt.Sprintf("%s:%v", path, args)
	}
	now := time.Now()
	ent, ok := cachedCalls[key]
	if ok && now.Sub(ent.ts) <= ttl {
		return ent.res, nil
	}
	res, err := Get(path, args...)
	if err != nil {
		return nil, err
	}
	cachedCalls[key] = cacheEntry{
		ts: now,
		res: res,
	}
	return res, nil
}

func Account() (*models.Account, error) {
	res, err := cacheGET("", 0, "accounts/me")
	if err != nil {
		return nil, err
	}
	return models.ParseAccount(res)
}

func ReplicantID(id int) (string, error) {
	account, err := Account()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s-%d", account.Name, id)
	var names []string
	for _, r := range account.Replicants {
		if r.Name == name {
			return r.ReplicantCode, nil
		}
		names = append(names, r.Name)
	}
	return "", fmt.Errorf("no replicant %q found in %q", name, names)
}

func ReplicantScan(id string) (*models.Scan, error) {
	res, err := cachePOST("", 0, "replicants/%s/scan", nil, id)
	if err != nil {
		return nil, err
	}
	return models.ParseScan(res)
}

func Replicant(id string) (*models.Replicant, error) {
	res, err := cacheGET("", 0, "replicants/%s", id)
	if err != nil {
		return nil, err
	}
	return models.ParseReplicant(res)
}

func Travel(id, dest string) (*models.Trip, error) {
	if dest == "" || id == "" {
		return nil, fmt.Errorf("id and destination are required for travel")
	}
	data, _ := json.Marshal(map[string]string{
		"destination": dest,
	})
	trip, err := Post("replicants/%s/travel", data, id)
	if err != nil {
		return nil, err
	}
	return models.ParseTrip(trip)
}
