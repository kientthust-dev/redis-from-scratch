package command

import (
	"errors"
	"math/rand"

	"redis_core/internal/config"
)

const (
	evictionPolicyRandom = "random"
	evictionPolicyLRU    = "lru"
)

var errCapacityUnavailable = errors.New("unable to make room for a new key")

// evictRandom removes one randomly selected key from the store.
// Expired keys are removed before selecting a candidate.
func evictRandom() bool {
	purgeExpiredKeys()

	var candidate string
	count := 0

	// Reservoir sampling gives every key an equal probability
	// of being selected without allocating a slice of all keys.
	for key := range store {
		count++

		if rand.Intn(count) == 0 {
			candidate = key
		}
	}

	if count == 0 {
		return false
	}

	delete(store, candidate)
	return true
}

// evictLRU removes the least recently accessed key among
// a randomly selected sample of keys.
func evictLRU(sampleCount int) bool {
	if sampleCount <= 0 {
		return false
	}

	purgeExpiredKeys()

	candidates := make([]string, 0, sampleCount)
	count := 0

	// Reservoir sampling selects up to sampleCount distinct keys.
	for key := range store {
		count++

		if len(candidates) < sampleCount {
			candidates = append(candidates, key)
			continue
		}

		index := rand.Intn(count)
		if index < sampleCount {
			candidates[index] = key
		}
	}

	if len(candidates) == 0 {
		return false
	}

	// Find the key with the oldest lastAccess timestamp.
	oldestKey := candidates[0]
	oldestAccess := store[oldestKey].lastAccess

	for _, key := range candidates[1:] {
		lastAccess := store[key].lastAccess

		// A zero timestamp is treated as the oldest access time.
		if lastAccess.IsZero() ||
			(!oldestAccess.IsZero() && lastAccess.Before(oldestAccess)) {
			oldestKey = key
			oldestAccess = lastAccess
		}
	}

	delete(store, oldestKey)
	return true
}

// ensureCapacity makes room for one new key if the store is full.
// It returns true if a new key can be inserted.
func ensureCapacity() bool {
	purgeExpiredKeys()

	if len(store) < config.MaxKeys {
		return true
	}

	switch config.EvictionPolicy {
	case evictionPolicyRandom:
		return evictRandom()

	case evictionPolicyLRU:
		return evictLRU(config.EvictionSamples)

	default:
		return false
	}
}
