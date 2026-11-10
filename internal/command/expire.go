package command

import (
	"redis_core/internal/protocol"
	"strconv"
	"time"
)

func (cmd *Command) handleExpire() ([]byte, error) {
	switch cmd.Name {
	case "TTL":
		return cmd.ttl()
	case "EXPIRE":
		return cmd.expire()
	default:
		return nil, ErrUnknownCommand
	}
}

func (cmd *Command) ttl() ([]byte, error) {
	if len(cmd.Args) != 1 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]
	now := time.Now()

	e, exists := store[key]
	if exists && e.hasExpiry && !now.Before(e.expireAt) {
		delete(store, key)
		exists = false
	}

	if !exists {
		return protocol.Encode(-2)
	}

	updateLastAccess(key)
	if !e.hasExpiry {
		return protocol.Encode(-1)
	}

	remaining := int64(e.expireAt.Sub(now).Seconds())
	if remaining < 0 {
		remaining = 0
	}

	return protocol.Encode(remaining)
}

func (cmd *Command) expire() ([]byte, error) {
	if len(cmd.Args) != 2 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]

	seconds, err := strconv.ParseInt(cmd.Args[1], 10, 64)
	if err != nil {
		return nil, ErrInvalidCommand
	}

	now := time.Now()

	e, exists := store[key]
	if exists && e.hasExpiry && !now.Before(e.expireAt) {
		delete(store, key)
		exists = false
	}

	if !exists {
		return protocol.Encode(0)
	}

	if seconds <= 0 {
		updateLastAccess(key)
		delete(store, key)
		return protocol.Encode(1)
	}

	e.expireAt = now.Add(time.Duration(seconds) * time.Second)
	e.hasExpiry = true
	e.lastAccess = now
	store[key] = e

	return protocol.Encode(1)
}
