package command

import (
	"errors"
	"strconv"
	"strings"
	"time"

	data_structure "redis_core/internal/data_structure"
	"redis_core/internal/protocol"
)

var errBloomWrongType = errors.New(
	"WRONGTYPE Operation against a key holding the wrong kind of value",
)

// handleBloom dispatches Bloom Filter commands.
func (cmd *Command) handleBloom() ([]byte, error) {
	switch strings.ToUpper(cmd.Name) {
	case "BF.RESERVE":
		return cmd.bfReserve()

	case "BF.ADD":
		return cmd.bfAdd()

	case "BF.EXISTS":
		return cmd.bfExists()

	case "BF.MADD":
		return cmd.bfMAdd()

	case "BF.MEXISTS":
		return cmd.bfMExists()

	default:
		return nil, ErrUnknownCommand
	}
}

// lookupBloom retrieves a Bloom Filter from the store.
//
// It returns (nil, false, false) if the key does not exist.
// It returns (nil, true, true) if the key has the wrong data type.
func lookupBloom(key string) (*data_structure.Bloom, bool, bool) {
	e, exists := store[key]
	if !exists {
		return nil, false, false
	}

	if e.hasExpiry && !time.Now().Before(e.expireAt) {
		delete(store, key)
		return nil, false, false
	}

	if e.dataType != TypeBloomFilter {
		return nil, true, true
	}

	bloom, ok := e.data.(*data_structure.Bloom)
	if !ok || bloom == nil {
		return nil, true, true
	}

	return bloom, true, false
}

// encodeBloomError converts a Go error into a RESP error.
//
// WRONGTYPE errors do not receive the "ERR " prefix.
// Other Bloom Filter errors receive the "ERR " prefix.
func encodeBloomError(err error) ([]byte, error) {
	message := err.Error()

	if !errors.Is(err, errBloomWrongType) {
		message = "ERR " + message
	}

	return protocol.Encode(protocol.RespError(message))
}

// bfReserve initializes a Bloom Filter.
//
// Syntax:
//
//	BF.RESERVE key errorRate capacity
func (cmd *Command) bfReserve() ([]byte, error) {
	if len(cmd.Args) != 3 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]

	errorRate, err := strconv.ParseFloat(cmd.Args[1], 64)
	if err != nil {
		return protocol.Encode(
			protocol.RespError("ERR invalid error rate"),
		)
	}

	capacity, err := strconv.ParseUint(cmd.Args[2], 10, 64)
	if err != nil {
		return protocol.Encode(
			protocol.RespError("ERR invalid capacity"),
		)
	}

	if keyExists(key) {
		return protocol.Encode(
			protocol.RespError("ERR item exists"),
		)
	}

	bloom, err := data_structure.CreateBloomFilter(capacity, errorRate)
	if err != nil {
		return protocol.Encode(
			protocol.RespError("ERR " + err.Error()),
		)
	}

	if !ensureCapacity() {
		return encodeCapacityError()
	}

	store[key] = entry{
		data:       bloom,
		dataType:   TypeBloomFilter,
		lastAccess: time.Now(),
	}

	return protocol.Encode("OK")
}

// getOrCreateBloom retrieves an existing Bloom Filter or creates
// one using default parameters if the key does not exist.
//
// Default capacity: 100
// Default error rate: 0.01
func getOrCreateBloom(key string) (*data_structure.Bloom, error) {
	bloom, exists, wrongType := lookupBloom(key)

	if wrongType {
		return nil, errBloomWrongType
	}

	if exists {
		return bloom, nil
	}

	bloom, err := data_structure.CreateBloomFilter(100, 0.01)
	if err != nil {
		return nil, err
	}

	if !ensureCapacity() {
		return nil, errCapacityUnavailable
	}

	store[key] = entry{
		data:       bloom,
		dataType:   TypeBloomFilter,
		lastAccess: time.Now(),
	}

	return bloom, nil
}

// bfAdd adds one element to a Bloom Filter.
//
// Syntax:
//
//	BF.ADD key item
//
// Returns 1 if the item is probably new.
// Returns 0 if the item may already exist.
func (cmd *Command) bfAdd() ([]byte, error) {
	if len(cmd.Args) != 2 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]
	item := cmd.Args[1]

	bloom, err := getOrCreateBloom(key)
	if err != nil {
		return encodeBloomError(err)
	}

	added := bloom.AddIfNotExist(item)
	updateLastAccess(key)

	if added {
		return protocol.Encode(int64(1))
	}

	return protocol.Encode(int64(0))
}

// bfExists checks whether an element may exist in a Bloom Filter.
//
// Syntax:
//
//	BF.EXISTS key item
//
// Returns 1 if the item may exist.
// Returns 0 if the item definitely does not exist,
// or if the Bloom Filter key does not exist.
func (cmd *Command) bfExists() ([]byte, error) {
	if len(cmd.Args) != 2 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]
	item := cmd.Args[1]

	bloom, exists, wrongType := lookupBloom(key)

	if wrongType {
		return protocol.Encode(
			protocol.RespError(errBloomWrongType.Error()),
		)
	}

	if !exists {
		return protocol.Encode(int64(0))
	}

	updateLastAccess(key)
	if bloom.Exist(item) {
		return protocol.Encode(int64(1))
	}

	return protocol.Encode(int64(0))
}

// bfMAdd adds multiple elements to a Bloom Filter.
//
// Syntax:
//
//	BF.MADD key item [item ...]
//
// Returns an array of integers.
func (cmd *Command) bfMAdd() ([]byte, error) {
	if len(cmd.Args) < 2 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]

	bloom, err := getOrCreateBloom(key)
	if err != nil {
		return encodeBloomError(err)
	}

	results := make([]interface{}, 0, len(cmd.Args)-1)

	for _, item := range cmd.Args[1:] {
		added := bloom.AddIfNotExist(item)

		if added {
			results = append(results, int64(1))
		} else {
			results = append(results, int64(0))
		}
	}

	updateLastAccess(key)
	return protocol.Encode(results)
}

// bfMExists checks whether multiple elements may exist.
//
// Syntax:
//
//	BF.MEXISTS key item [item ...]
//
// Returns an array of integers.
func (cmd *Command) bfMExists() ([]byte, error) {
	if len(cmd.Args) < 2 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]

	bloom, exists, wrongType := lookupBloom(key)

	if wrongType {
		return protocol.Encode(
			protocol.RespError(errBloomWrongType.Error()),
		)
	}

	results := make([]interface{}, 0, len(cmd.Args)-1)

	if !exists {
		for range cmd.Args[1:] {
			results = append(results, int64(0))
		}

		return protocol.Encode(results)
	}

	updateLastAccess(key)
	for _, item := range cmd.Args[1:] {
		if bloom.Exist(item) {
			results = append(results, int64(1))
		} else {
			results = append(results, int64(0))
		}
	}

	return protocol.Encode(results)
}
