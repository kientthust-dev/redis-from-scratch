package command

import (
	"fmt"
	"testing"
	"time"

	"redis_core/internal/config"
)

// resetStore replaces the global store for an isolated test.
// The original store is restored when the test finishes.
func resetStore(t *testing.T) {
	t.Helper()

	originalStore := store
	store = make(map[string]entry)

	t.Cleanup(func() {
		store = originalStore
	})
}

// Test evictRandom with multiple keys.
// Exactly one key should be removed.
func TestEvictRandom(t *testing.T) {
	resetStore(t)

	store["key1"] = entry{}
	store["key2"] = entry{}
	store["key3"] = entry{}

	before := len(store)

	ok := evictRandom()

	if !ok {
		t.Fatal("evictRandom() returned false, expected true")
	}

	if got := len(store); got != before-1 {
		t.Fatalf("expected %d keys after eviction, got %d", before-1, got)
	}
}

// Test evictRandom with an empty store.
func TestEvictRandomEmptyStore(t *testing.T) {
	resetStore(t)

	if evictRandom() {
		t.Fatal("evictRandom() returned true for an empty store")
	}

	if len(store) != 0 {
		t.Fatalf("expected empty store, got %d keys", len(store))
	}
}

// Test that evictRandom removes expired keys before eviction.
func TestEvictRandomPurgesExpiredKeys(t *testing.T) {
	resetStore(t)

	store["expired"] = entry{
		hasExpiry: true,
		expireAt:  time.Now().Add(-time.Hour),
	}

	if evictRandom() {
		t.Fatal("expected false because expired keys should be purged first")
	}

	if len(store) != 0 {
		t.Fatalf("expected empty store after purging, got %d keys", len(store))
	}
}

// Test LRU eviction with all keys included in the sample.
// The key with the oldest lastAccess should be removed.
func TestEvictLRU(t *testing.T) {
	resetStore(t)

	now := time.Now()

	store["oldest"] = entry{
		lastAccess: now.Add(-3 * time.Hour),
	}

	store["middle"] = entry{
		lastAccess: now.Add(-2 * time.Hour),
	}

	store["newest"] = entry{
		lastAccess: now.Add(-time.Hour),
	}

	ok := evictLRU(len(store))

	if !ok {
		t.Fatal("evictLRU() returned false, expected true")
	}

	if _, exists := store["oldest"]; exists {
		t.Fatal("expected oldest key to be evicted")
	}

	if len(store) != 2 {
		t.Fatalf("expected 2 keys after eviction, got %d", len(store))
	}
}

// Test that a zero lastAccess timestamp is treated as the oldest.
func TestEvictLRUZeroTimestamp(t *testing.T) {
	resetStore(t)

	store["zero"] = entry{}

	store["recent"] = entry{
		lastAccess: time.Now(),
	}

	ok := evictLRU(len(store))

	if !ok {
		t.Fatal("evictLRU() returned false, expected true")
	}

	if _, exists := store["zero"]; exists {
		t.Fatal("expected key with zero timestamp to be evicted")
	}

	if _, exists := store["recent"]; !exists {
		t.Fatal("expected recently accessed key to remain")
	}
}

// Test LRU with a sample count larger than the number of keys.
func TestEvictLRUSampleLargerThanStore(t *testing.T) {
	resetStore(t)

	store["key1"] = entry{lastAccess: time.Now()}
	store["key2"] = entry{lastAccess: time.Now().Add(-time.Hour)}

	ok := evictLRU(100)

	if !ok {
		t.Fatal("evictLRU() returned false, expected true")
	}

	if len(store) != 1 {
		t.Fatalf("expected 1 key after eviction, got %d", len(store))
	}
}

// Test LRU with an invalid sample count.
func TestEvictLRUInvalidSampleCount(t *testing.T) {
	resetStore(t)

	store["key1"] = entry{}

	if evictLRU(0) {
		t.Fatal("expected false for sampleCount = 0")
	}

	if evictLRU(-1) {
		t.Fatal("expected false for negative sampleCount")
	}

	if len(store) != 1 {
		t.Fatal("store should not change for invalid sample count")
	}
}

// Test that LRU purges expired keys before selecting a candidate.
func TestEvictLRUPurgesExpiredKeys(t *testing.T) {
	resetStore(t)

	store["expired"] = entry{
		hasExpiry: true,
		expireAt:  time.Now().Add(-time.Hour),
	}

	store["valid"] = entry{
		lastAccess: time.Now(),
	}

	ok := evictLRU(10)

	if !ok {
		t.Fatal("expected valid key to be evicted")
	}

	if len(store) != 0 {
		t.Fatalf("expected empty store, got %d keys", len(store))
	}
}

// Test ensureCapacity when the store has available capacity.
func TestEnsureCapacityAvailable(t *testing.T) {
	resetStore(t)

	store["key1"] = entry{}

	if !ensureCapacity() {
		t.Fatal("ensureCapacity() returned false although capacity is available")
	}

	if len(store) != 1 {
		t.Fatalf("expected 1 key, got %d", len(store))
	}
}

// Test ensureCapacity when the store reaches MaxKeys.
// The default configuration uses the random eviction policy.
func TestEnsureCapacityAtMaxKeys(t *testing.T) {
	resetStore(t)

	if config.MaxKeys <= 0 {
		t.Skip("MaxKeys must be positive for this test")
	}

	for i := 0; i < config.MaxKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		store[key] = entry{}
	}

	if len(store) != config.MaxKeys {
		t.Fatalf("expected %d keys, got %d", config.MaxKeys, len(store))
	}

	ok := ensureCapacity()

	if !ok {
		t.Fatal("ensureCapacity() returned false at maximum capacity")
	}

	if len(store) != config.MaxKeys-1 {
		t.Fatalf(
			"expected %d keys after eviction, got %d",
			config.MaxKeys-1,
			len(store),
		)
	}
}

// Test ensureCapacity purges expired keys before checking capacity.
// An expired key should free capacity without evicting a valid key.
func TestEnsureCapacityPurgesExpiredKeys(t *testing.T) {
	resetStore(t)

	store["expired"] = entry{
		hasExpiry: true,
		expireAt:  time.Now().Add(-time.Hour),
	}

	store["valid"] = entry{}

	if !ensureCapacity() {
		t.Fatal("expected capacity to be available after purging expired key")
	}

	if len(store) != 1 {
		t.Fatalf("expected 1 remaining key, got %d", len(store))
	}

	if _, exists := store["valid"]; !exists {
		t.Fatal("valid key should not be evicted")
	}

	if _, exists := store["expired"]; exists {
		t.Fatal("expired key should have been removed")
	}
}
