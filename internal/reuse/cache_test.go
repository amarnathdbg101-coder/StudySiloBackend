package reuse

import (
	"testing"
	"time"
)

func TestMemoryCache_DeleteByPrefix(t *testing.T) {
	cache := GetCache()
	cache.Flush()

	cache.Set("smart_ctx:user1:gate:socratic", "res1", 5*time.Minute)
	cache.Set("smart_ctx:user1:jee:feynman", "res2", 5*time.Minute)
	cache.Set("smart_ctx:user2:neet:cram", "res3", 5*time.Minute)
	cache.Set("other:data", "val", 5*time.Minute)

	// Invalidate user1 entries
	cache.DeleteByPrefix("smart_ctx:user1:")

	if _, found := cache.Get("smart_ctx:user1:gate:socratic"); found {
		t.Fatalf("expected smart_ctx:user1:gate:socratic to be deleted")
	}
	if _, found := cache.Get("smart_ctx:user1:jee:feynman"); found {
		t.Fatalf("expected smart_ctx:user1:jee:feynman to be deleted")
	}
	if _, found := cache.Get("smart_ctx:user2:neet:cram"); !found {
		t.Fatalf("expected smart_ctx:user2:neet:cram to still exist")
	}
	if _, found := cache.Get("other:data"); !found {
		t.Fatalf("expected other:data to still exist")
	}
}
