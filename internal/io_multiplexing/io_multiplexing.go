package io_multiplexing

type Operation int32

const (
	OpRead Operation = 1 << iota
	OpWrite
)

type Event struct {
	Fd       int
	Op       Operation
	IsWakeup bool
}

type IOMultiplexer interface {
	Monitor(event Event) error
	Update(event Event) error
	Remove(fd int) error
	Wakeup() error
	Wait() ([]Event, error)
	Close() error
}
