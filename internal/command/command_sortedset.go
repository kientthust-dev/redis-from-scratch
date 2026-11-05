package command

import (
	"math"
	"strconv"
	"strings"
	"time"

	data_structure "redis_core/internal/data_structure"
	"redis_core/internal/protocol"
)

// handleSortedSet dispatches sorted-set commands.
func (cmd *Command) handleSortedSet() ([]byte, error) {
	switch strings.ToUpper(cmd.Name) {
	case "ZADD":
		return cmd.zadd()
	case "ZSCORE":
		return cmd.zscore()
	case "ZRANK":
		return cmd.zrank()
	default:
		return nil, ErrUnknownCommand
	}
}

func (cmd *Command) zadd() ([]byte, error) {
	if len(cmd.Args) < 3 || len(cmd.Args)%2 == 0 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]

	type pair struct {
		score  float64
		member string
	}

	pairs := make([]pair, 0, (len(cmd.Args)-1)/2)

	// Validate every score before modifying the store.
	for i := 1; i < len(cmd.Args); i += 2 {
		score, err := strconv.ParseFloat(cmd.Args[i], 64)
		if err != nil || math.IsNaN(score) || math.IsInf(score, 0) {
			return protocol.Encode(protocol.RespError("ERR value is not a valid float"))
		}

		pairs = append(pairs, pair{
			score:  score,
			member: cmd.Args[i+1],
		})
	}

	now := time.Now()
	var added int64

	e, exists := store[key]

	if exists && e.hasExpiry && !now.Before(e.expireAt) {
		delete(store, key)
		exists = false
	}

	if exists && e.dataType != TypeSortedSet {
		return protocol.Encode(protocol.RespError(
			"WRONGTYPE Operation against a key holding the wrong kind of value",
		))
	}

	var ss *data_structure.SortedSet

	if exists {
		var ok bool
		ss, ok = e.data.(*data_structure.SortedSet)
		if !ok || ss == nil {
			return nil, ErrInvalidCommand
		}
	} else {
		ss = data_structure.NewSortedSet()
		e = entry{
			data:     ss,
			dataType: TypeSortedSet,
		}
	}

	for _, p := range pairs {
		if ss.Add(p.score, p.member) {
			added++
		}
	}

	store[key] = e
	return protocol.Encode(added)
}

func (cmd *Command) zscore() ([]byte, error) {
	if len(cmd.Args) != 2 {
		return nil, ErrWrongNumberOfArguments
	}

	key, member := cmd.Args[0], cmd.Args[1]
	now := time.Now()

	e, exists := store[key]
	if !exists {
		return protocol.Encode(nil)
	}

	if e.hasExpiry && !now.Before(e.expireAt) {
		delete(store, key)
		return protocol.Encode(nil)
	}

	if e.dataType != TypeSortedSet {
		return protocol.Encode(protocol.RespError(
			"WRONGTYPE Operation against a key holding the wrong kind of value",
		))
	}

	ss, ok := e.data.(*data_structure.SortedSet)
	if !ok || ss == nil {
		return nil, ErrInvalidCommand
	}

	score, exists := ss.Score(member)
	if !exists {
		return protocol.Encode(nil)
	}

	return protocol.Encode([]byte(strconv.FormatFloat(score, 'f', -1, 64)))
}

func (cmd *Command) zrank() ([]byte, error) {
	if len(cmd.Args) != 2 {
		return nil, ErrWrongNumberOfArguments
	}

	key, member := cmd.Args[0], cmd.Args[1]
	now := time.Now()

	e, exists := store[key]
	if !exists {
		return protocol.Encode(nil)
	}

	if e.hasExpiry && !now.Before(e.expireAt) {
		delete(store, key)
		return protocol.Encode(nil)
	}

	if e.dataType != TypeSortedSet {
		return protocol.Encode(protocol.RespError(
			"WRONGTYPE Operation against a key holding the wrong kind of value",
		))
	}

	ss, ok := e.data.(*data_structure.SortedSet)
	if !ok || ss == nil {
		return nil, ErrInvalidCommand
	}

	rank, exists := ss.Rank(member)
	if !exists {
		return protocol.Encode(nil)
	}

	return protocol.Encode(int64(rank))
}
