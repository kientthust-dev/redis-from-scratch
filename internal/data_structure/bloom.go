package data_structure

import (
	"errors"
	"math"
)

const (
	ln2       = 0.6931471805599453
	ln2Square = 0.4804530139182014

	// FNV-1a offset basis values for two independent hash streams.
	fnvOffset1 uint64 = 14695981039346656037
	fnvOffset2 uint64 = 1099511628211
	fnvPrime   uint64 = 1099511628211
)

var (
	ErrInvalidCapacity  = errors.New("bloom filter capacity must be greater than zero")
	ErrInvalidErrorRate = errors.New("bloom filter error rate must be between 0 and 1")
	ErrFilterTooLarge   = errors.New("bloom filter is too large")
)

// Bloom represents a Bloom Filter.
//
// It uses a bit array and multiple hash functions to test
// whether an element may exist in a set.
//
// False positives are possible, but false negatives are not.
type Bloom struct {
	Hashes  int
	Entries uint64
	Error   float64

	bitPerEntry float64
	bf          []uint8
	bits        uint64
}

// CreateBloomFilter creates a Bloom Filter with the given capacity
// and desired false-positive probability.
//
// capacity: expected number of elements.
// errorRate: desired false-positive probability, between 0 and 1.
//
// The optimal bit array size is:
//
//	m = -n * ln(p) / (ln(2)^2)
//
// The optimal number of hash functions is:
//
//	k = (m / n) * ln(2)
func CreateBloomFilter(capacity uint64, errorRate float64) (*Bloom, error) {
	if capacity == 0 {
		return nil, ErrInvalidCapacity
	}

	if math.IsNaN(errorRate) ||
		math.IsInf(errorRate, 0) ||
		errorRate <= 0 ||
		errorRate >= 1 {
		return nil, ErrInvalidErrorRate
	}

	// Calculate the number of bits required per entry.
	bitPerEntry := -math.Log(errorRate) / ln2Square

	// Calculate the total number of bits.
	requiredBits := math.Ceil(float64(capacity) * bitPerEntry)

	maxInt := uint64(^uint(0) >> 1)

	// Ensure the allocation size can fit into an int.
	if math.IsInf(requiredBits, 0) ||
		requiredBits < 1 ||
		requiredBits > float64(maxInt)*8 {
		return nil, ErrFilterTooLarge
	}

	bitSize := uint64(requiredBits)

	// Round the number of bits up to a whole number of bytes.
	byteSize := (bitSize-1)/8 + 1

	if byteSize > maxInt {
		return nil, ErrFilterTooLarge
	}

	// Calculate the optimal number of hash functions.
	hashCount := int(math.Round(bitPerEntry * ln2))

	if hashCount < 1 {
		hashCount = 1
	}

	if hashCount > int(maxInt) {
		return nil, ErrFilterTooLarge
	}

	return &Bloom{
		Hashes:      hashCount,
		Entries:     capacity,
		Error:       errorRate,
		bitPerEntry: bitPerEntry,
		bf:          make([]uint8, int(byteSize)),
		bits:        bitSize,
	}, nil
}

// hashPair generates two hash values for an element.
//
// Double hashing uses these two values to generate k hash positions:
//
//	h(i) = (h1 + i*h2) % m
//
// This avoids having to execute k independent hash functions.
func hashPair(element string) (uint64, uint64) {
	h1 := fnvOffset1
	h2 := fnvOffset2

	for i := 0; i < len(element); i++ {
		value := uint64(element[i])

		h1 ^= value
		h1 *= fnvPrime

		// Use a different initial value and domain separator
		// for the second hash stream.
		h2 ^= value + 1
		h2 *= fnvPrime
	}

	// Keep the second hash odd to reduce repeated positions
	// when generating hash indices.
	h2 |= 1

	return h1, h2
}

// Add inserts an element into the Bloom Filter.
//
// It sets all bits corresponding to the element's hash positions.
func (b *Bloom) Add(element string) {
	if b == nil || b.bits == 0 || len(b.bf) == 0 {
		return
	}

	h1, h2 := hashPair(element)

	for i := 0; i < b.Hashes; i++ {
		position := (h1 + uint64(i)*h2) % b.bits

		bytePos := position / 8
		bitPos := position % 8

		b.bf[bytePos] |= uint8(1 << bitPos)
	}
}

// Exist checks whether an element may exist in the Bloom Filter.
//
// Returns false if at least one corresponding bit is zero.
// Returns true if all corresponding bits are one.
//
// A true result does not guarantee that the element exists,
// because false positives are possible.
func (b *Bloom) Exist(element string) bool {
	if b == nil || b.bits == 0 || len(b.bf) == 0 {
		return false
	}

	h1, h2 := hashPair(element)

	for i := 0; i < b.Hashes; i++ {
		position := (h1 + uint64(i)*h2) % b.bits

		bytePos := position / 8
		bitPos := position % 8

		mask := uint8(1 << bitPos)

		if b.bf[bytePos]&mask == 0 {
			return false
		}
	}

	return true
}

// AddIfNotExist checks whether an element may already exist.
//
// If Exist returns false, the element is added and the method
// returns true.
//
// If Exist returns true, the method returns false.
//
// Note that a false positive may cause this method to return
// false even if the element was never added before.
func (b *Bloom) AddIfNotExist(element string) bool {
	if b == nil {
		return false
	}

	if b.Exist(element) {
		return false
	}

	b.Add(element)
	return true
}

// BitSize returns the number of bits in the Bloom Filter.
func (b *Bloom) BitSize() uint64 {
	if b == nil {
		return 0
	}

	return b.bits
}

// MemoryUsage returns the allocated bit-array size in bytes.
func (b *Bloom) MemoryUsage() uint64 {
	if b == nil {
		return 0
	}

	return uint64(len(b.bf))
}

// HashCount returns the number of hash functions.
func (b *Bloom) HashCount() int {
	if b == nil {
		return 0
	}

	return b.Hashes
}
