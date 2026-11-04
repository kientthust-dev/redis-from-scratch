package command

// import (
// 	"strconv"
// 	"strings"
// 	"time"

// 	data_structure "redis_core/internal/data_structure"
// 	"redis_core/internal/protocol"
// )

// // handleCMS dispatches CMS commands.
// func (cmd *Command) handleCMS() ([]byte, error) {
// 	switch strings.ToUpper(cmd.Name) {
// 	case "CMS.INITBYPROB":
// 		return cmd.cmsInitByProb()

// 	case "CMS.INITBYDIM":
// 		return cmd.cmsInitByDim()

// 	case "CMS.INCRBY":
// 		return cmd.cmsIncrBy()

// 	case "CMS.QUERY":
// 		return cmd.cmsQuery()

// 	default:
// 		return nil, ErrUnknownCommand
// 	}
// }

// func (cmd *Command) cmsInitByProb() ([]byte, error) {
// 	if len(cmd.Args) != 3 {
// 		return nil, ErrWrongNumberOfArguments
// 	}

// 	key := cmd.Args[0]

// 	errorRate, err := strconv.ParseFloat(cmd.Args[1], 64)
// 	if err != nil {
// 		return nil, ErrInvalidCommand
// 	}

// 	probability, err := strconv.ParseFloat(cmd.Args[2], 64)
// 	if err != nil {
// 		return nil, ErrInvalidCommand
// 	}
