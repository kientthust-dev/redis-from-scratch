//go:build linux

package io_multiplexing

import "syscall"

func (e Event) toNative() syscall.EpollEvent {
	var nativeEvent syscall.EpollEvent
	nativeEvent.Fd = int32(e.Fd)

	if e.Op&OpRead != 0 {
		nativeEvent.Events |= syscall.EPOLLIN
	}

	if e.Op&OpWrite != 0 {
		nativeEvent.Events |= syscall.EPOLLOUT
	}

	return nativeEvent
}

func createEvent(ep syscall.EpollEvent) Event {
	var op Operation

	if ep.Events&syscall.EPOLLIN != 0 {
		op |= OpRead
	}

	if ep.Events&syscall.EPOLLOUT != 0 {
		op |= OpWrite
	}

	return Event{
		Fd: int(ep.Fd),
		Op: op,
	}
}
