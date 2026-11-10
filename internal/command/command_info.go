package command

import (
	"fmt"

	"redis_core/internal/protocol"
)

const infoKeyspaceFormat = "# Keyspace\r\ndb0:keys=%d,expires=%d,avg_ttl=%d\r\n"

// handleInfo handles the INFO command.
func (cmd *Command) handleInfo() ([]byte, error) {
	if len(cmd.Args) != 0 {
		return nil, ErrWrongNumberOfArguments
	}

	keys, expires, avgTTL := getKeyspaceStats()

	info := fmt.Sprintf(
		infoKeyspaceFormat,
		keys,
		expires,
		avgTTL,
	)

	return protocol.Encode([]byte(info))
}
