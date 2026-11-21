package services

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"goProjectStructure/internal/handler/dto"
)

// cacheEntry holds the cached diagram response and its expiration metadata.
type cacheEntry struct {
	diagram   dto.DiagramResponse
	expiresAt time.Time
}

// DiagramCache provides a thread-safe, in-memory LRU-like TTL cache for generated AI mind maps.
// It avoids repeated expensive LLM calls and offloads compute from the VPS.
type DiagramCache struct {
	mu       sync.RWMutex
	capacity int
	ttl      time.Duration
	items    map[string]cacheEntry
	keys     []string // Track key insertion order for FIFO/LRU eviction when full
}

// NewDiagramCache initializes a DiagramCache with a fixed capacity and expiration duration.
func NewDiagramCache(capacity int, ttl time.Duration) *DiagramCache {
	if capacity <= 0 {
		capacity = 250 // Holds ~250 full academic diagrams (~1.5 MB RAM)
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour // Diagrams stay fresh for 24 hours
	}
	return &DiagramCache{
		capacity: capacity,
		ttl:      ttl,
		items:    make(map[string]cacheEntry, capacity),
		keys:     make([]string, 0, capacity),
	}
}

// BuildCacheKey creates a deterministic SHA-256 hash from canonical concept, archetype, and language.
// Normalizing case and whitespace ensures equivalent student queries hit the same cache entry.
func BuildCacheKey(concept, archetype, language string) string {
	normConcept := strings.ToLower(strings.TrimSpace(concept))
	normArchetype := strings.ToLower(strings.TrimSpace(archetype))
	normLang := strings.ToLower(strings.TrimSpace(language))

	raw := normConcept + ":" + normArchetype + ":" + normLang
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:16]) // First 16 bytes (32 hex chars) is collision-free for cache keys
}

// Get retrieves a cached DiagramResponse if it exists and has not expired.
// Uses sync.RWMutex.RLock() so multiple concurrent HTTP reader goroutines do NOT block each other.
func (c *DiagramCache) Get(key string) (*dto.DiagramResponse, bool) {
	c.mu.RLock()
	entry, found := c.items[key]
	c.mu.RUnlock()

	if !found {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		// Lazily remove expired item under write lock
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false
	}

	// Return a deep copy to prevent caller goroutines from mutating cached slices/pointers.
	// Common Go concurrency pitfall: returning a pointer to cached struct allows concurrent
	// requests to mutate diagram.Nodes concurrently, causing silent data races!
	clone := cloneDiagram(entry.diagram)
	return &clone, true
}

// Set stores a generated DiagramResponse under write lock with TTL and capacity management.
func (c *DiagramCache) Set(key string, diagram dto.DiagramResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If capacity reached and key is new, evict the oldest key
	if len(c.items) >= c.capacity {
		if _, exists := c.items[key]; !exists && len(c.keys) > 0 {
			oldest := c.keys[0]
			c.keys = c.keys[1:]
			delete(c.items, oldest)
		}
	}

	// Update or insert
	if _, exists := c.items[key]; !exists {
		c.keys = append(c.keys, key)
	}

	c.items[key] = cacheEntry{
		diagram:   cloneDiagram(diagram),
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Clear flushes all cached entries.
func (c *DiagramCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]cacheEntry, c.capacity)
	c.keys = c.keys[:0]
}

// Len returns current number of active cached items.
func (c *DiagramCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// cloneDiagram produces a deep copy of dto.DiagramResponse to ensure total thread safety.
func cloneDiagram(src dto.DiagramResponse) dto.DiagramResponse {
	dst := src

	if src.Nodes != nil {
		dst.Nodes = make([]dto.GraphNode, len(src.Nodes))
		copy(dst.Nodes, src.Nodes)
		// Deep copy maps and slices inside nodes
		for i := range dst.Nodes {
			if src.Nodes[i].Metrics != nil {
				dst.Nodes[i].Metrics = make(map[string]string, len(src.Nodes[i].Metrics))
				for k, v := range src.Nodes[i].Metrics {
					dst.Nodes[i].Metrics[k] = v
				}
			}
			if src.Nodes[i].Formulas != nil {
				dst.Nodes[i].Formulas = make([]string, len(src.Nodes[i].Formulas))
				copy(dst.Nodes[i].Formulas, src.Nodes[i].Formulas)
			}
			if src.Nodes[i].CrossLinks != nil {
				dst.Nodes[i].CrossLinks = make([]string, len(src.Nodes[i].CrossLinks))
				copy(dst.Nodes[i].CrossLinks, src.Nodes[i].CrossLinks)
			}
		}
	}

	if src.Edges != nil {
		dst.Edges = make([]dto.GraphEdge, len(src.Edges))
		copy(dst.Edges, src.Edges)
	}

	return dst
}
