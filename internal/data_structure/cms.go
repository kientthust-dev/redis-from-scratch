package data_structure

import (
	"errors"
	"math"

	"github.com/spaolacci/murmur3"
)

const (
	MaxCMSCounters uint64 = 10_000_000
)

type CountMinSketch struct {
	// Two-dimensional counter matrix, counters[depth][width]
	counters [][]uint32

	depth uint32
	width uint32

	epsilon float64
	delta   float64
}

// NewCountMinSketch initializes CMS using error rate and probability.
func NewCountMinSketch(epsilon, delta float64) (*CountMinSketch, error) {
	if math.IsNaN(epsilon) || math.IsInf(epsilon, 0) ||
		math.IsNaN(delta) || math.IsInf(delta, 0) {
		return nil, errors.New("epsilon and delta must be finite")
	}

	if epsilon <= 0 || delta <= 0 || epsilon >= 1 || delta >= 1 {
		return nil, errors.New("epsilon and delta must be in (0, 1)")
	}

	width := uint64(math.Ceil(math.E / epsilon))
	depth := uint64(math.Ceil(math.Log(1 / delta)))

	return newCountMinSketch(width, depth, epsilon, delta)
}

// NewCountMinSketchByDim initializes CMS using width and depth.
func NewCountMinSketchByDim(width, depth uint32) (*CountMinSketch, error) {
	if width == 0 || depth == 0 {
		return nil, errors.New("CMS width and depth must be greater than zero")
	}

	return newCountMinSketch(uint64(width), uint64(depth), 0, 0)
}

// newCountMinSketch allocates the counter matrix after validating dimensions.
func newCountMinSketch(
	width, depth uint64,
	epsilon, delta float64,
) (*CountMinSketch, error) {
	if width == 0 || depth == 0 {
		return nil, errors.New("CMS width and depth must be greater than zero")
	}

	if width > MaxCMSCounters/depth {
		return nil, errors.New("CMS allocation exceeds counter limit")
	}

	if width > uint64(math.MaxInt) || depth > uint64(math.MaxInt) {
		return nil, errors.New("CMS dimensions exceed int limit")
	}

	counters := make([][]uint32, int(depth))

	for i := range counters {
		counters[i] = make([]uint32, int(width))
	}

	return &CountMinSketch{
		counters: counters,
		depth:    uint32(depth),
		width:    uint32(width),
		epsilon:  epsilon,
		delta:    delta,
	}, nil
}

func (cms *CountMinSketch) hashIndexes(item string) []uint32 {
	hash1, hash2 := murmur3.Sum128([]byte(item))

	indexes := make([]uint32, cms.depth)

	width := uint64(cms.width)
	base := hash1 % width
	step := hash2 % width

	if step == 0 {
		step = 1
	}

	for i := uint32(0); i < cms.depth; i++ {
		indexes[i] = uint32(base)
		base = (base + step) % width
	}

	return indexes
}

func (cms *CountMinSketch) IncrBy(item string, value uint32) uint32 {
	indexes := cms.hashIndexes(item)

	minCount := uint32(math.MaxUint32)

	for row, index := range indexes {
		currentCount := cms.counters[row][index]

		if currentCount > math.MaxUint32-value {
			currentCount = math.MaxUint32
		} else {
			currentCount += value
		}

		cms.counters[row][index] = currentCount

		if currentCount < minCount {
			minCount = currentCount
		}
	}

	return minCount
}

func (cms *CountMinSketch) Count(item string) uint32 {
	indexes := cms.hashIndexes(item)

	minCount := uint32(math.MaxUint32)

	for row, index := range indexes {
		currentCount := cms.counters[row][index]

		if currentCount < minCount {
			minCount = currentCount
		}
	}

	return minCount
}
