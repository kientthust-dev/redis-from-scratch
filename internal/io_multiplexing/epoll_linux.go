package io_multiplexing

import (
	"encoding/binary"
	"errors"
	"io"
	"redis_core/internal/config"
	"syscall"
)

type Epoll struct {
	epfd          int
	wakeupFD      int
	epollEvents   []syscall.EpollEvent
	genericEvents []Event
}

var _ IOMultiplexer = (*Epoll)(nil)

func CreateIOMultiplexer() (IOMultiplexer, error) {
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		return nil, err
	}

	fd, _, errno := syscall.Syscall(
		syscall.SYS_EVENTFD2,
		0,
		uintptr(syscall.O_NONBLOCK|syscall.O_CLOEXEC),
		0,
	)
	if errno != 0 {
		_ = syscall.Close(epfd)
		return nil, errno
	}

	wakeupFD := int(fd)

	nativeEvent := syscall.EpollEvent{
		Events: syscall.EPOLLIN,
		Fd:     int32(wakeupFD),
	}

	if err := syscall.EpollCtl(
		epfd,
		syscall.EPOLL_CTL_ADD,
		wakeupFD,
		&nativeEvent,
	); err != nil {
		_ = syscall.Close(wakeupFD)
		_ = syscall.Close(epfd)
		return nil, err
	}

	return &Epoll{
		epfd:          epfd,
		wakeupFD:      wakeupFD,
		epollEvents:   make([]syscall.EpollEvent, config.MaxEvents),
		genericEvents: make([]Event, config.MaxEvents),
	}, nil
}

func (ep *Epoll) Monitor(event Event) error {
	nativeEvent := event.toNative()

	return syscall.EpollCtl(
		ep.epfd,
		syscall.EPOLL_CTL_ADD,
		event.Fd,
		&nativeEvent,
	)
}

func (ep *Epoll) Update(event Event) error {
	nativeEvent := event.toNative()

	return syscall.EpollCtl(
		ep.epfd,
		syscall.EPOLL_CTL_MOD,
		event.Fd,
		&nativeEvent,
	)
}

func (ep *Epoll) Remove(fd int) error {
	return syscall.EpollCtl(
		ep.epfd,
		syscall.EPOLL_CTL_DEL,
		fd,
		nil,
	)
}

func (ep *Epoll) Wakeup() error {
	var buf [8]byte
	binary.NativeEndian.PutUint64(buf[:], 1)

	for {
		n, err := syscall.Write(ep.wakeupFD, buf[:])

		if err == syscall.EINTR {
			continue
		}

		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			// The eventfd is already readable.
			return nil
		}

		if err != nil {
			return err
		}

		if n != len(buf) {
			return io.ErrShortWrite
		}

		return nil
	}
}

func (ep *Epoll) drainWakeupFD() error {
	var buf [8]byte

	for {
		n, err := syscall.Read(ep.wakeupFD, buf[:])

		if err == syscall.EINTR {
			continue
		}

		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			return nil
		}

		if err != nil {
			return err
		}

		if n != len(buf) {
			return io.ErrUnexpectedEOF
		}

		// A successful eventfd read resets its counter.
		return nil
	}
}

func (ep *Epoll) Wait() ([]Event, error) {
	for {
		n, err := syscall.EpollWait(
			ep.epfd,
			ep.epollEvents,
			-1,
		)

		if err != nil {
			if err == syscall.EINTR {
				continue
			}

			return nil, err
		}

		for i := 0; i < n; i++ {
			if int(ep.epollEvents[i].Fd) == ep.wakeupFD {
				if err := ep.drainWakeupFD(); err != nil {
					return nil, err
				}

				ep.genericEvents[i] = Event{
					IsWakeup: true,
				}
				continue
			}

			ep.genericEvents[i] = createEvent(ep.epollEvents[i])
		}

		return ep.genericEvents[:n], nil
	}
}

func (ep *Epoll) Close() error {
	var closeErr error

	if ep.epfd >= 0 {
		if err := syscall.Close(ep.epfd); err != nil {
			closeErr = err
		}
		ep.epfd = -1
	}

	if ep.wakeupFD >= 0 {
		if err := syscall.Close(ep.wakeupFD); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
		ep.wakeupFD = -1
	}

	return closeErr
}
