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
