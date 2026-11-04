package command

import (
	"redis_core/internal/protocol"
	"time"
)

func (cmd *Command) handleKey() ([]byte, error) {
	switch cmd.Name {
	case "DEL":
		return cmd.del()
	case "EXISTS":
		return cmd.exists()
	default:
		return nil, ErrUnknownCommand
	}
}

func (cmd *Command) del() ([]byte, error) {
	if len(cmd.Args) == 0 {
		return nil, ErrWrongNumberOfArguments
	}

	now := time.Now()
	deleted := int64(0)

	store.mu.Lock()
	defer store.mu.Unlock()

	for _, key := range cmd.Args {
		e, exists := store.data[key]
		if !exists {
			continue
		}

		if e.hasExpiry && !now.Before(e.expireAt) {
			delete(store.data, key)
			continue
		}

		delete(store.data, key)
		deleted++
	}

	return protocol.Encode(deleted)
}

func (cmd *Command) exists() ([]byte, error) {
	if len(cmd.Args) == 0 {
		return nil, ErrWrongNumberOfArguments
	}

	now := time.Now()
	count := int64(0)

	store.mu.Lock()
	defer store.mu.Unlock()

	for _, key := range cmd.Args {
		e, exists := store.data[key]
		if !exists {
			continue
		}

		if e.hasExpiry && !now.Before(e.expireAt) {
			delete(store.data, key)
			continue
		}

		count++
	}

	return protocol.Encode(count)
}
