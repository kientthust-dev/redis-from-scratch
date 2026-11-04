package command

import (
	"strings"
	"time"

	data_structure "redis_core/internal/data_structure"
	"redis_core/internal/protocol"
)

func (cmd *Command) handleSet() ([]byte, error) {
	switch strings.ToUpper(cmd.Name) {
	case "SADD":
		return cmd.sadd()
	case "SREM":
		return cmd.srem()
	case "SISMEMBER":
		return cmd.sismember()
	case "SMEMBERS":
		return cmd.smembers()
	default:
		return nil, ErrUnknownCommand
	}
}

func (cmd *Command) sadd() ([]byte, error) {
	if len(cmd.Args) < 2 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]
	members := cmd.Args[1:]
	now := time.Now()
	var added int64

	store.mu.Lock()
	defer store.mu.Unlock()

	e, exists := store.data[key]

	if exists && e.hasExpiry && !now.Before(e.expireAt) {
		delete(store.data, key)
		exists = false
	}

	if exists && e.dataType != TypeSet {
		return protocol.Encode(protocol.RespError(
			"WRONGTYPE Operation against a key holding the wrong kind of value",
		))
	}

	var set *data_structure.Set

	if exists {
		var ok bool
		set, ok = e.data.(*data_structure.Set)
		if !ok || set == nil {
			return nil, ErrInvalidCommand
		}
	} else {
		set = data_structure.NewSet()
		e = entry{
			data:     set,
			dataType: TypeSet,
		}
	}

	for _, member := range members {
		if set.Add(member) {
			added++
		}
	}

	store.data[key] = e
	return protocol.Encode(added)
}

func (cmd *Command) srem() ([]byte, error) {
	if len(cmd.Args) < 2 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]
	members := cmd.Args[1:]
	now := time.Now()
	var removed int64

	store.mu.Lock()
	defer store.mu.Unlock()

	e, exists := store.data[key]
	if !exists {
		return protocol.Encode(removed)
	}

	if e.hasExpiry && !now.Before(e.expireAt) {
		delete(store.data, key)
		return protocol.Encode(removed)
	}

	if e.dataType != TypeSet {
		return protocol.Encode(protocol.RespError(
			"WRONGTYPE Operation against a key holding the wrong kind of value",
		))
	}

	set, ok := e.data.(*data_structure.Set)
	if !ok || set == nil {
		return nil, ErrInvalidCommand
	}

	for _, member := range members {
		if set.Remove(member) {
			removed++
		}
	}

	if set.Len() == 0 {
		delete(store.data, key)
	}

	return protocol.Encode(removed)
}

func (cmd *Command) sismember() ([]byte, error) {
	if len(cmd.Args) != 2 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]
	member := cmd.Args[1]
	now := time.Now()

	store.mu.Lock()
	defer store.mu.Unlock()

	e, exists := store.data[key]
	if !exists {
		return protocol.Encode(int64(0))
	}

	if e.hasExpiry && !now.Before(e.expireAt) {
		delete(store.data, key)
		return protocol.Encode(int64(0))
	}

	if e.dataType != TypeSet {
		return protocol.Encode(protocol.RespError(
			"WRONGTYPE Operation against a key holding the wrong kind of value",
		))
	}

	set, ok := e.data.(*data_structure.Set)
	if !ok || set == nil {
		return nil, ErrInvalidCommand
	}

	if set.Contains(member) {
		return protocol.Encode(int64(1))
	}

	return protocol.Encode(int64(0))
}

func (cmd *Command) smembers() ([]byte, error) {
	if len(cmd.Args) != 1 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]
	now := time.Now()

	store.mu.Lock()
	defer store.mu.Unlock()

	e, exists := store.data[key]
	if !exists {
		return protocol.Encode([]interface{}{})
	}

	if e.hasExpiry && !now.Before(e.expireAt) {
		delete(store.data, key)
		return protocol.Encode([]interface{}{})
	}

	if e.dataType != TypeSet {
		return protocol.Encode(protocol.RespError(
			"WRONGTYPE Operation against a key holding the wrong kind of value",
		))
	}

	set, ok := e.data.(*data_structure.Set)
	if !ok || set == nil {
		return nil, ErrInvalidCommand
	}

	rawMembers := set.Members()
	members := make([]interface{}, 0, len(rawMembers))

	for _, member := range rawMembers {
		members = append(members, []byte(member))
	}

	return protocol.Encode(members)
}
