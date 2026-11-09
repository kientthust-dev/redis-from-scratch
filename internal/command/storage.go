package command

import "time"

type DataType uint8

const (
	TypeString DataType = iota
	TypeSet
	TypeSortedSet
	TypeBloomFilter
	TypeCountMinSketch
)

type entry struct {
	data      interface{}
	dataType  DataType
	expireAt  time.Time
	hasExpiry bool
}

var store = make(map[string]entry)

// getKeyspaceStats returns the number of valid keys,
// the number of keys with expiration, and their average remaining TTL
// in milliseconds.
func getKeyspaceStats() (keys, expires int, avgTTL int64) {
	now := time.Now()
	var totalTTL int64

	for key, item := range store {
		// Remove expired keys from the store.
		if item.hasExpiry && !item.expireAt.After(now) {
			delete(store, key)
			continue
		}

		keys++

		if item.hasExpiry {
			expires++
			totalTTL += item.expireAt.Sub(now).Milliseconds()
		}
	}

	if expires > 0 {
		avgTTL = totalTTL / int64(expires)
	}

	return keys, expires, avgTTL
}
