package services

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"goProjectStructure/internal/handler/dto"
)

func TestDiagramCache_BasicGetSet(t *testing.T) {
	cache := NewDiagramCache(10, 1*time.Hour)

	diag := dto.DiagramResponse{
		Concept:   "Binary Search",
		Archetype: "flow",
		Nodes: []dto.GraphNode{
			{ID: "n1", Label: "Mid Calculation", Metrics: map[string]string{"Complexity": "O(1)"}},
		},
	}

	key := BuildCacheKey("Binary Search", "flow", "hinglish")
	cache.Set(key, diag)

	// Fetch from cache
	cached, found := cache.Get(key)
	if !found || cached == nil {
		t.Fatalf("Expected key %q to be in cache", key)
	}
	if cached.Concept != "Binary Search" {
		t.Errorf("Expected concept 'Binary Search', got %q", cached.Concept)
	}

	// Verify deep copy: mutating returned object must NOT mutate the cache
	cached.Nodes[0].Label = "MUTATED"
	freshFetch, _ := cache.Get(key)
	if freshFetch.Nodes[0].Label == "MUTATED" {
		t.Errorf("Cache was mutated! Deep copy isolation failed.")
	}
}

func TestDiagramCache_TTLExpiration(t *testing.T) {
	// Ultra short TTL of 20ms
	cache := NewDiagramCache(5, 20*time.Millisecond)

	diag := dto.DiagramResponse{Concept: "Transient Concept"}
	key := BuildCacheKey("Transient Concept", "tree", "en")
	cache.Set(key, diag)

	// Immediate fetch should succeed
	if _, found := cache.Get(key); !found {
		t.Fatalf("Expected item to be present immediately")
	}

	// Sleep 35ms for TTL expiry
	time.Sleep(35 * time.Millisecond)

	// Item must now report not found
	if _, found := cache.Get(key); found {
		t.Errorf("Expected item to be expired and not found after TTL")
	}
}

func TestDiagramCache_CapacityEviction(t *testing.T) {
	cache := NewDiagramCache(2, 1*time.Hour)

	cache.Set("key1", dto.DiagramResponse{Concept: "Topic 1"})
	cache.Set("key2", dto.DiagramResponse{Concept: "Topic 2"})
	// 3rd item should evict key1
	cache.Set("key3", dto.DiagramResponse{Concept: "Topic 3"})

	if _, found := cache.Get("key1"); found {
		t.Errorf("Expected key1 to be evicted when capacity reached")
	}
	if _, found := cache.Get("key2"); !found {
		t.Errorf("Expected key2 to still exist")
	}
	if _, found := cache.Get("key3"); !found {
		t.Errorf("Expected key3 to exist")
	}
}

func TestDiagramCache_ConcurrentAccessStress(t *testing.T) {
	cache := NewDiagramCache(50, 1*time.Hour)
	const numGoroutines = 60
	const numOpsPerGoroutine = 40

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(gID int) {
			defer wg.Done()
			for op := 0; op < numOpsPerGoroutine; op++ {
				key := fmt.Sprintf("topic_%d", op%15) // Deliberately overlapping keys to stress lock
				if (gID+op)%3 == 0 {
					// Write
					cache.Set(key, dto.DiagramResponse{
						Concept: fmt.Sprintf("Concept %d", op),
						Nodes: []dto.GraphNode{
							{ID: fmt.Sprintf("node_%d", op), Label: "Test", Metrics: map[string]string{"k": "v"}},
						},
					})
				} else {
					// Read
					cached, found := cache.Get(key)
					if found && cached != nil {
						// Read slice and map to test race conditions
						_ = len(cached.Nodes)
						if len(cached.Nodes) > 0 {
							_ = cached.Nodes[0].Metrics["k"]
						}
					}
				}
			}
		}(g)
	}

	wg.Wait()

	if cache.Len() == 0 {
		t.Errorf("Expected cache to contain items after concurrent stress test")
	}
}
