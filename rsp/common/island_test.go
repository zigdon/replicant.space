package common

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestIslandCacheBasic(t *testing.T) {
	cache := NewIslandCache()

	// 1. Miss on empty cache
	if _, ok := cache.Get("SOL", 7.5, 100); ok {
		t.Errorf("Expected cache miss on empty cache")
	}

	// 2. Put an island result
	info := &IslandInfo{
		TargetStar: "ISLAND_A",
		Hop:        7.5,
		Limit:      100,
		Stars:      []string{"ISLAND_A", "ISLAND_B", "ISLAND_C"},
		StarMap: map[string]bool{
			"ISLAND_A": true,
			"ISLAND_B": true,
			"ISLAND_C": true,
		},
		CalculatedAt: time.Now(),
	}
	cache.Put(info)

	// 3. Hit on target star
	got, ok := cache.Get("ISLAND_A", 7.5, 100)
	if !ok || got == nil {
		t.Fatalf("Expected cache hit for ISLAND_A")
	}
	if len(got.Stars) != 3 {
		t.Errorf("Expected 3 stars in island, got %d", len(got.Stars))
	}

	// 4. Hit on member star (ISLAND_B and ISLAND_C)
	gotB, okB := cache.Get("ISLAND_B", 7.5, 100)
	if !okB || gotB == nil {
		t.Errorf("Expected cache hit for member star ISLAND_B")
	}
	if gotB != got {
		t.Errorf("Expected member star to point to same IslandInfo instance")
	}

	gotC, okC := cache.Get("ISLAND_C", 7.5, 100)
	if !okC || gotC == nil {
		t.Errorf("Expected cache hit for member star ISLAND_C")
	}

	// 5. Miss on different hop or limit
	if _, ok := cache.Get("ISLAND_A", 10.0, 100); ok {
		t.Errorf("Expected cache miss for different hop distance")
	}
	if _, ok := cache.Get("ISLAND_A", 7.5, 50); ok {
		t.Errorf("Expected cache miss for different limit")
	}

	// 6. Contains helper
	if !info.Contains("ISLAND_B") {
		t.Errorf("Contains(ISLAND_B) should return true")
	}
	if !info.Contains("island_b") {
		t.Errorf("Contains(island_b) case-insensitive should return true")
	}
	if info.Contains("OTHER") {
		t.Errorf("Contains(OTHER) should return false")
	}

	// 7. Clear cache
	cache.Clear()
	if _, ok := cache.Get("ISLAND_A", 7.5, 100); ok {
		t.Errorf("Expected cache miss after Clear()")
	}
}

func TestIslandCacheConcurrent(t *testing.T) {
	cache := NewIslandCache()
	var wg sync.WaitGroup

	// Concurrently write and read multiple islands
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			starA := fmt.Sprintf("STAR_%d_A", idx)
			starB := fmt.Sprintf("STAR_%d_B", idx)
			info := &IslandInfo{
				TargetStar: starA,
				Hop:        7.5,
				Limit:      100,
				Stars:      []string{starA, starB},
				StarMap:    map[string]bool{starA: true, starB: true},
			}
			cache.Put(info)

			got, ok := cache.Get(starA, 7.5, 100)
			if !ok || got == nil || len(got.Stars) != 2 {
				t.Errorf("Concurrent Get failed for %s", starA)
			}
		}(i)
	}

	wg.Wait()
}

func TestIslandCacheGlobal(t *testing.T) {
	ClearIslandCache()

	info := &IslandInfo{
		TargetStar: "SOL_TEST",
		Hop:        7.5,
		Limit:      100,
		Stars:      []string{"SOL_TEST"},
		StarMap:    map[string]bool{"SOL_TEST": true},
	}
	globalIslandCache.Put(info)

	got, ok := GetCachedIsland("SOL_TEST", 7.5, 100)
	if !ok || got == nil || got.TargetStar != "SOL_TEST" {
		t.Errorf("GetCachedIsland failed")
	}

	ClearIslandCache()
	if _, ok := GetCachedIsland("SOL_TEST", 7.5, 100); ok {
		t.Errorf("Expected miss after ClearIslandCache")
	}
}
