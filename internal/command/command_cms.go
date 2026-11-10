package command

import (
	"strconv"
	"strings"
	"time"

	data_structure "redis_core/internal/data_structure"
	"redis_core/internal/protocol"
)

// handleCMS dispatches CMS commands.
func (cmd *Command) handleCMS() ([]byte, error) {
	switch strings.ToUpper(cmd.Name) {
	case "CMS.INITBYPROB":
		return cmd.cmsInitByProb()

	case "CMS.INITBYDIM":
		return cmd.cmsInitByDim()

	case "CMS.INCRBY":
		return cmd.cmsIncrBy()

	case "CMS.QUERY":
		return cmd.cmsQuery()

	case "CMS.INFO":
		return cmd.cmsInfo()

	default:
		return nil, ErrUnknownCommand
	}
}

// keyExists checks whether a key exists and removes it if expired.
func keyExists(key string) bool {
	e, exists := store[key]
	if !exists {
		return false
	}

	if e.hasExpiry && !time.Now().Before(e.expireAt) {
		delete(store, key)
		return false
	}

	return true
}

// lookupCMS retrieves a Count-Min Sketch from the store.
// It returns (nil, false, false) if the key does not exist.
// It returns (nil, true, true) if the key has the wrong data type.
func lookupCMS(key string) (*data_structure.CountMinSketch, bool, bool) {
	e, exists := store[key]
	if !exists {
		return nil, false, false
	}

	if e.hasExpiry && !time.Now().Before(e.expireAt) {
		delete(store, key)
		return nil, false, false
	}

	if e.dataType != TypeCountMinSketch {
		return nil, true, true
	}

	cms, ok := e.data.(*data_structure.CountMinSketch)
	if !ok || cms == nil {
		return nil, true, true
	}

	return cms, true, false
}

// cmsInitByProb initializes a CMS using error rate and probability.
//
// Syntax:
// CMS.INITBYPROB key errorRate probability
func (cmd *Command) cmsInitByProb() ([]byte, error) {
	if len(cmd.Args) != 3 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]

	errorRate, err := strconv.ParseFloat(cmd.Args[1], 64)
	if err != nil {
		return nil, ErrInvalidCommand
	}

	probability, err := strconv.ParseFloat(cmd.Args[2], 64)
	if err != nil {
		return nil, ErrInvalidCommand
	}

	if keyExists(key) {
		return protocol.Encode(
			protocol.RespError("CMS: key already exists"),
		)
	}

	cms, err := data_structure.NewCountMinSketch(errorRate, probability)
	if err != nil {
		return protocol.Encode(
			protocol.RespError("ERR " + err.Error()),
		)
	}

	if !ensureCapacity() {
		return encodeCapacityError()
	}

	store[key] = entry{
		data:       cms,
		dataType:   TypeCountMinSketch,
		lastAccess: time.Now(),
	}

	return protocol.Encode("OK")
}

// cmsInitByDim initializes a CMS using width and depth.
//
// Syntax:
// CMS.INITBYDIM key width depth
func (cmd *Command) cmsInitByDim() ([]byte, error) {
	if len(cmd.Args) != 3 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]

	width, err := strconv.ParseUint(cmd.Args[1], 10, 32)
	if err != nil {
		return nil, ErrInvalidCommand
	}

	depth, err := strconv.ParseUint(cmd.Args[2], 10, 32)
	if err != nil {
		return nil, ErrInvalidCommand
	}

	if keyExists(key) {
		return protocol.Encode(
			protocol.RespError("CMS: key already exists"),
		)
	}

	cms, err := data_structure.NewCountMinSketchByDim(
		uint32(width),
		uint32(depth),
	)
	if err != nil {
		return protocol.Encode(
			protocol.RespError("ERR " + err.Error()),
		)
	}

	if !ensureCapacity() {
		return encodeCapacityError()
	}

	store[key] = entry{
		data:       cms,
		dataType:   TypeCountMinSketch,
		lastAccess: time.Now(),
	}

	return protocol.Encode("OK")
}

// cmsIncrBy increments the estimated frequency of one or more items.
//
// Syntax:
// CMS.INCRBY key item increment [item increment ...]
func (cmd *Command) cmsIncrBy() ([]byte, error) {
	if len(cmd.Args) < 3 || (len(cmd.Args)-1)%2 != 0 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]

	cms, exists, wrongType := lookupCMS(key)

	if !exists {
		return protocol.Encode(
			protocol.RespError("CMS: key does not exist"),
		)
	}

	if wrongType {
		return protocol.Encode(protocol.RespError(wrongTypeError))
	}

	// Parse and validate all increments before modifying the CMS.
	type increment struct {
		item  string
		value uint32
	}

	increments := make([]increment, 0, (len(cmd.Args)-1)/2)

	for i := 1; i < len(cmd.Args); i += 2 {
		item := cmd.Args[i]

		value, err := strconv.ParseUint(cmd.Args[i+1], 10, 32)
		if err != nil || value == 0 {
			return nil, ErrInvalidCommand
		}

		increments = append(increments, increment{
			item:  item,
			value: uint32(value),
		})
	}

	updateLastAccess(key)

	// Apply all increments after successful validation.
	results := make([]interface{}, 0, len(increments))

	for _, inc := range increments {
		count := cms.IncrBy(inc.item, inc.value)
		results = append(results, int64(count))
	}

	return protocol.Encode(results)
}

// cmsQuery returns the estimated frequency of one or more items.
//
// Syntax:
// CMS.QUERY key item [item ...]
func (cmd *Command) cmsQuery() ([]byte, error) {
	if len(cmd.Args) < 2 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]

	cms, exists, wrongType := lookupCMS(key)

	if !exists {
		return protocol.Encode(
			protocol.RespError("CMS: key does not exist"),
		)
	}

	if wrongType {
		return protocol.Encode(protocol.RespError(wrongTypeError))
	}

	updateLastAccess(key)
	results := make([]interface{}, 0, len(cmd.Args)-1)

	for _, item := range cmd.Args[1:] {
		count := cms.Count(item)
		results = append(results, int64(count))
	}

	return protocol.Encode(results)
}

// cmsInfo returns information about a Count-Min Sketch.
//
// Syntax:
// CMS.INFO key
func (cmd *Command) cmsInfo() ([]byte, error) {
	if len(cmd.Args) != 1 {
		return nil, ErrWrongNumberOfArguments
	}

	key := cmd.Args[0]

	cms, exists, wrongType := lookupCMS(key)

	if !exists {
		return protocol.Encode(
			protocol.RespError("CMS: key does not exist"),
		)
	}

	if wrongType {
		return protocol.Encode(protocol.RespError(wrongTypeError))
	}

	updateLastAccess(key)
	width, depth, count := cms.Info()

	results := []interface{}{
		"width", int64(width),
		"depth", int64(depth),
		"count", int64(count),
	}

	return protocol.Encode(results)
}
