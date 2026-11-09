package command

import (
	"fmt"

	"redis_core/internal/protocol"
)

// handleInfo handles the INFO command.
func (cmd *Command) handleInfo() ([]byte, error) {
	if len(cmd.Args) != 0 {
		return nil, ErrWrongNumberOfArguments
	}

	keys, expires, avgTTL := getKeyspaceStats()

	info := fmt.Sprintf(
		"# Keyspace\r\ndb0:keys=%d,expires=%d,avg_ttl=%d\r\n",
		keys,
		expires,
		avgTTL,
	)

	return protocol.Encode([]byte(info))
}
