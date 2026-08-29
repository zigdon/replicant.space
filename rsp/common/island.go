package common

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// IslandInfo represents the calculated relay island details for a star system.
type IslandInfo struct {
	TargetStar    string
	Hop           float32
	Limit         int
	Stars         []string
	StarMap       map[string]bool
	IsNetwork     bool // True if the star is already in or reachable from the relay network
	LimitExceeded bool // True if island exceeds size limit
	Err           error
	CalculatedAt  time.Time
}

// Contains returns true if the specified star is part of this island.
func (info *IslandInfo) Contains(star string) bool {
	if info == nil || info.StarMap == nil {
		return false
	}
	return info.StarMap[strings.ToUpper(strings.TrimSpace(star))]
}

// IslandCache provides thread-safe in-memory caching and deduplication for relay island calculations.
type IslandCache struct {
	mu       sync.RWMutex
	entries  map[string]*IslandInfo
	inFlight map[string]chan struct{}
}

// NewIslandCache creates a new empty IslandCache.
func NewIslandCache() *IslandCache {
	return &IslandCache{
		entries:  make(map[string]*IslandInfo),
		inFlight: make(map[string]chan struct{}),
	}
}

func cacheKey(star string, hop float32, limit int) string {
	return fmt.Sprintf("%s:%.2f:%d", strings.ToUpper(strings.TrimSpace(star)), hop, limit)
}

// Get retrieves a cached IslandInfo for the given star, hop, and limit.
func (c *IslandCache) Get(star string, hop float32, limit int) (*IslandInfo, bool) {
	if c == nil {
		return nil, false
	}
	key := cacheKey(star, hop, limit)
	c.mu.RLock()
	defer c.mu.RUnlock()
	info, ok := c.entries[key]
	return info, ok
}

// Put stores an IslandInfo in the cache. It registers the result for all stars in the island cluster.
func (c *IslandCache) Put(info *IslandInfo) {
	if c == nil || info == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store for target star
	c.entries[cacheKey(info.TargetStar, info.Hop, info.Limit)] = info

	// Store for all stars in the island so any member star lookup hits cache
	if len(info.Stars) > 0 {
		for _, s := range info.Stars {
			c.entries[cacheKey(s, info.Hop, info.Limit)] = info
		}
	}
}

// Clear clears all entries in the cache.
func (c *IslandCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*IslandInfo)
	c.inFlight = make(map[string]chan struct{})
}

// CalculateOrGet retrieves the cached island or calculates it using RelayIsland if not already cached.
// In-flight requests for the same parameters are deduplicated to avoid redundant calculations.
func (c *IslandCache) CalculateOrGet(loc string, hop float32, limit int) (*IslandInfo, error) {
	loc = strings.ToUpper(strings.TrimSpace(loc))
	if loc == "" {
		return nil, fmt.Errorf("star location is required")
	}
	if hop <= 0 {
		hop = 7.5
	}
	if limit <= 0 {
		limit = 100
	}

	key := cacheKey(loc, hop, limit)

	// 1. Fast path: check cache
	c.mu.RLock()
	if info, ok := c.entries[key]; ok {
		c.mu.RUnlock()
		return info, info.Err
	}

	// 2. Check if a calculation is already in flight
	ch, waiting := c.inFlight[key]
	if !waiting {
		ch = make(chan struct{})
		c.inFlight[key] = ch
	}
	c.mu.RUnlock()

	if waiting {
		// Wait for existing in-flight calculation
		<-ch
		c.mu.RLock()
		info := c.entries[key]
		c.mu.RUnlock()
		if info != nil {
			return info, info.Err
		}
		return nil, fmt.Errorf("failed to retrieve island result for %s", loc)
	}

	// 3. Perform calculation
	stars, err := RelayIsland(loc, hop, limit)

	info := &IslandInfo{
		TargetStar:   loc,
		Hop:          hop,
		Limit:        limit,
		CalculatedAt: time.Now(),
	}

	if err != nil {
		info.Err = err
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "relay network") {
			info.IsNetwork = true
		}
		if strings.Contains(errStr, "limit") {
			info.LimitExceeded = true
		}
	} else {
		info.Stars = stars
		info.StarMap = make(map[string]bool, len(stars))
		for _, s := range stars {
			info.StarMap[strings.ToUpper(strings.TrimSpace(s))] = true
		}
	}

	// 4. Update cache and signal waiting goroutines
	c.Put(info)

	c.mu.Lock()
	delete(c.inFlight, key)
	close(ch)
	c.mu.Unlock()

	return info, err
}

// Global default cache instance
var globalIslandCache = NewIslandCache()

// GetOrCalculateIsland retrieves or calculates relay island using the default global cache.
func GetOrCalculateIsland(loc string, hop float32, limit int) (*IslandInfo, error) {
	return globalIslandCache.CalculateOrGet(loc, hop, limit)
}

// GetCachedIsland returns a cached island if present without triggering calculation.
func GetCachedIsland(loc string, hop float32, limit int) (*IslandInfo, bool) {
	return globalIslandCache.Get(loc, hop, limit)
}

// ClearIslandCache clears the global island cache.
func ClearIslandCache() {
	globalIslandCache.Clear()
}
