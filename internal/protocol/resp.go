package protocol

import (
	"bytes"
	"errors"
	"strconv"
)

var (
	ErrInvalidResp        = errors.New("invalid resp message")
	ErrNeedMoreData       = errors.New("need more data")
	ErrBulkStringTooLarge = errors.New("bulk string exceeds maximum size")
	ErrArrayTooLarge      = errors.New("array exceeds maximum size")
	ErrNestingTooDeep     = errors.New("RESP nesting exceeds maximum depth")
)

const (
	CRLF              string = "\r\n"
	NilReply                 = "$-1\r\n"
	MaxBulkStringSize        = 512 * 1024 * 1024 // 512MB
	MaxArraySize             = 100_000
	MaxNestingDepth          = 128
)

// RespError represents a RESP error reply.
type RespError string

// Parse parses one RESP value from the available bytes.
// consumed is the number of bytes belonging to the parsed value.
// ErrNeedMoreData means more bytes are needed to complete the value.
func Parse(data []byte) (value interface{}, consumed int, err error) {
	return parse(data, 0)
}

func parse(data []byte, depth int) (interface{}, int, error) {
	if len(data) == 0 {
		return nil, 0, ErrNeedMoreData
	}

	switch data[0] {
	case '+':
		return parseSimpleString(data[1:])
	case '-':
		return parseError(data[1:])
	case ':':
		return parseInteger(data[1:])
	case '$':
		return parseBulkString(data[1:])
	case '*':
		if depth >= MaxNestingDepth {
			return nil, 0, ErrNestingTooDeep
		}
		return parseArray(data[1:], depth+1)
	default:
		return parseInlineCommand(data)
	}
}

// parseInlineCommand parses an inline command terminated by CRLF.
func parseInlineCommand(data []byte) (interface{}, int, error) {
	// Find the end of the inline command.
	end := bytes.Index(data, []byte(CRLF))
	if end == -1 {
		// Wait for more data if CRLF has not arrived yet.
		if bytes.Contains(data, []byte("\n")) {
			return nil, 0, ErrInvalidResp
		}
		return nil, 0, ErrNeedMoreData
	}

	line := data[:end]

	if len(line) == 0 {
		return nil, 0, ErrInvalidResp
	}

	// Split the command into tokens.
	tokens := bytes.Fields(line)
	if len(tokens) == 0 {
		return nil, 0, ErrInvalidResp
	}

	// Convert tokens to the format expected by command.ParseCommand().
	result := make([]interface{}, len(tokens))
	for i, token := range tokens {
		result[i] = token
	}

	return result, end + len(CRLF), nil
}

// readLine reads a CRLF-terminated line.
// consumed includes CRLF but excludes any RESP prefix byte.
func readLine(data []byte) (line string, consumed int, err error) {
	for i, b := range data {
		if b == '\n' {
			if i == 0 || data[i-1] != '\r' {
				return "", 0, ErrInvalidResp
			}
			return string(data[:i-1]), i + 1, nil
		}

		if b == '\r' {
			if i+1 >= len(data) {
				return "", 0, ErrNeedMoreData
			}
			if data[i+1] != '\n' {
				return "", 0, ErrInvalidResp
			}
		}
	}

	return "", 0, ErrNeedMoreData
}

func parseSimpleString(data []byte) (interface{}, int, error) {
	line, n, err := readLine(data)
	if err != nil {
		return nil, 0, err
	}

	return line, 1 + n, nil
}

func parseError(data []byte) (interface{}, int, error) {
	line, n, err := readLine(data)
	if err != nil {
		return nil, 0, err
	}

	return RespError(line), 1 + n, nil
}

func parseInteger(data []byte) (interface{}, int, error) {
	line, n, err := readLine(data)
	if err != nil {
		return nil, 0, err
	}

	value, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return nil, 0, ErrInvalidResp
	}

	return value, 1 + n, nil
}

func parseBulkString(data []byte) (interface{}, int, error) {
	line, n, err := readLine(data)
	if err != nil {
		return nil, 0, err
	}

	length, err := strconv.Atoi(line)
	if err != nil {
		return nil, 0, ErrInvalidResp
	}

	if length == -1 {
		return nil, 1 + n, nil
	}
	if length < -1 {
		return nil, 0, ErrInvalidResp
	}
	if length > MaxBulkStringSize {
		return nil, 0, ErrBulkStringTooLarge
	}

	remaining := len(data) - n

	// Check lengths before indexing the payload or its CRLF.
	if remaining < length+2 {
		return nil, 0, ErrNeedMoreData
	}

	start := n
	end := start + length

	if data[end] != '\r' || data[end+1] != '\n' {
		return nil, 0, ErrInvalidResp
	}

	// make preserves a non-nil empty slice for a zero-length bulk string.
	result := make([]byte, length)
	copy(result, data[start:end])

	return result, 1 + n + length + 2, nil
}

func parseArray(data []byte, depth int) (interface{}, int, error) {
	line, n, err := readLine(data)
	if err != nil {
		return nil, 0, err
	}

	length, err := strconv.Atoi(line)
	if err != nil {
		return nil, 0, ErrInvalidResp
	}

	if length == -1 {
		return nil, 1 + n, nil
	}
	if length < -1 {
		return nil, 0, ErrInvalidResp
	}
	if length > MaxArraySize {
		return nil, 0, ErrArrayTooLarge
	}

	result := make([]interface{}, length)

	// n counts the array header after the '*' prefix.
	offset := n

	for i := 0; i < length; i++ {
		if offset >= len(data) {
			return nil, 0, ErrNeedMoreData
		}

		value, childConsumed, err := parse(data[offset:], depth)
		if err != nil {
			return nil, 0, err
		}

		result[i] = value
		offset += childConsumed
	}

	// Include the '*' prefix in the consumed byte count.
	return result, 1 + offset, nil
}

func Encode(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case string:
		return encodeSimpleString(v), nil
	case RespError:
		return encodeError(v), nil
	case int:
		return encodeInteger(int64(v)), nil
	case int32:
		return encodeInteger(int64(v)), nil
	case int64:
		return encodeInteger(v), nil
	case []byte:
		return encodeBulkString(v), nil
	case nil:
		return []byte(NilReply), nil
	case []interface{}:
		return encodeArray(v)
	default:
		return nil, ErrInvalidResp
	}
}

func encodeSimpleString(s string) []byte {
	return []byte("+" + s + CRLF)
}

func encodeError(e RespError) []byte {
	return []byte("-" + string(e) + CRLF)
}

func encodeInteger(i int64) []byte {
	return []byte(":" + strconv.FormatInt(i, 10) + CRLF)
}

func encodeBulkString(b []byte) []byte {
	if b == nil {
		return []byte(NilReply)
	}

	return []byte("$" + strconv.Itoa(len(b)) + CRLF + string(b) + CRLF)
}

func encodeArray(arr []interface{}) ([]byte, error) {
	if arr == nil {
		return []byte("*-1\r\n"), nil
	}

	var buf bytes.Buffer
	buf.WriteString("*")
	buf.WriteString(strconv.Itoa(len(arr)))
	buf.WriteString(CRLF)

	for _, elem := range arr {
		encodedElem, err := Encode(elem)
		if err != nil {
			return nil, err
		}
		buf.Write(encodedElem)
	}

	return buf.Bytes(), nil
}
