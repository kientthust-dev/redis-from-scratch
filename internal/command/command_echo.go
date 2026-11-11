package command

import "redis_core/internal/protocol"

func (cmd *Command) handleEcho() ([]byte, error) {
	if len(cmd.Args) != 1 {
		return nil, ErrWrongNumberOfArguments
	}

	response, err := protocol.Encode([]byte(cmd.Args[0]))
	if err != nil {
		return nil, err
	}

	return response, nil
}
