package command

import (
	"sync"
	"time"
)

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

type Storage struct {
	mu   sync.RWMutex
	data map[string]entry
}

func NewStorage() *Storage {
	return &Storage{
		data: make(map[string]entry),
	}
}

var store = NewStorage()
