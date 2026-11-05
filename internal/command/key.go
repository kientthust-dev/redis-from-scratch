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

	for _, key := range cmd.Args {
		e, exists := store[key]
		if !exists {
			continue
		}

		if e.hasExpiry && !now.Before(e.expireAt) {
			delete(store, key)
			continue
		}

		delete(store, key)
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

	for _, key := range cmd.Args {
		e, exists := store[key]
		if !exists {
			continue
		}

		if e.hasExpiry && !now.Before(e.expireAt) {
			delete(store, key)
			continue
		}

		count++
	}

	return protocol.Encode(count)
}
