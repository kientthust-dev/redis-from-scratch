package command

import (
	"redis_core/internal/protocol"
	"strings"
	"time"
)

func (cmd *Command) handleString() ([]byte, error) {
	switch strings.ToUpper(cmd.Name) {
	case "PING":
		return cmd.ping()
	case "SET":
		return cmd.set()
	case "GET":
		return cmd.get()
	default:
		return nil, ErrUnknownCommand
	}
}

func (cmd *Command) ping() ([]byte, error) {
	switch len(cmd.Args) {
	case 0:
		return protocol.Encode("PONG")
	case 1:
		return protocol.Encode([]byte(cmd.Args[0]))
	default:
		return nil, ErrWrongNumberOfArguments
	}
}

func (cmd *Command) set() ([]byte, error) {
	if len(cmd.Args) != 2 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]
	value := cmd.Args[1]

	store.mu.Lock()
	store.data[key] = entry{
		data:     value,
		dataType: TypeString,
	}
	store.mu.Unlock()

	return protocol.Encode("OK")
}

func (cmd *Command) get() ([]byte, error) {
	if len(cmd.Args) != 1 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]

	store.mu.Lock()

	e, exists := store.data[key]
	if exists && e.hasExpiry && !time.Now().Before(e.expireAt) {
		delete(store.data, key)
		exists = false
	}

	store.mu.Unlock()

	if !exists {
		return protocol.Encode(nil)
	}

	if e.dataType != TypeString {
		return protocol.Encode(protocol.RespError("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	value, ok := e.data.(string)
	if !ok {
		return nil, ErrInvalidCommand
	}

	return protocol.Encode([]byte(value))
}
