package command

import (
	"errors"
	"strings"
)

var (
	ErrInvalidCommand         = errors.New("invalid command")
	ErrWrongNumberOfArguments = errors.New("wrong number of arguments")
	ErrUnknownCommand         = errors.New("unknown command")
)

type Command struct {
	Name string
	Args []string
}

func ParseCommand(value interface{}) (*Command, error) {
	array, ok := value.([]interface{})
	if !ok || len(array) == 0 {
		return nil, ErrInvalidCommand
	}

	tokens := make([]string, len(array))

	for i, elem := range array {
		b, ok := elem.([]byte)
		if !ok {
			return nil, ErrInvalidCommand
		}

		tokens[i] = string(b)
	}

	if tokens[0] == "" {
		return nil, ErrInvalidCommand
	}

	return &Command{
		Name: strings.ToUpper(tokens[0]),
		Args: tokens[1:],
	}, nil
}

func (cmd *Command) Handle() ([]byte, error) {
	if cmd == nil || cmd.Name == "" {
		return nil, ErrInvalidCommand
	}

	switch strings.ToUpper(cmd.Name) {
	case "PING", "SET", "GET":
		return cmd.handleString()

	case "TTL", "EXPIRE":
		return cmd.handleExpire()

	case "DEL", "EXISTS":
		return cmd.handleKey()

	case "SADD", "SREM", "SISMEMBER", "SMEMBERS":
		return cmd.handleSet()

	case "ZADD", "ZSCORE", "ZRANK":
		return cmd.handleSortedSet()

	default:
		return nil, ErrUnknownCommand
	}
}
