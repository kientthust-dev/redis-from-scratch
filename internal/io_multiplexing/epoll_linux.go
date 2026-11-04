package io_multiplexing

import (
	"redis_core/internal/config"
	"syscall"
)

type Epoll struct {
	epfd          int
	epollEvents   []syscall.EpollEvent
	genericEvents []Event
}

var _ IOMultiplexer = (*Epoll)(nil)

func CreateIOMultiplexer() (IOMultiplexer, error) {
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &Epoll{
		epfd:          epfd,
		epollEvents:   make([]syscall.EpollEvent, config.MaxEvents),
		genericEvents: make([]Event, config.MaxEvents),
	}, nil
}

func (ep *Epoll) Monitor(event Event) error {
	nativeEvent := event.toNative()
	err := syscall.EpollCtl(ep.epfd, syscall.EPOLL_CTL_ADD, event.Fd, &nativeEvent)
	if err != nil {
		return err
	}
	return nil
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

func (ep *Epoll) Wait() ([]Event, error) {
	for {
		n, err := syscall.EpollWait(
			ep.epfd,
			ep.epollEvents,
			-1,
		)

		if err != nil {
			if err == syscall.EINTR {
				// Retry if interrupted by a signal
				continue
			}

			return nil, err
		}

		for i := 0; i < n; i++ {
			ep.genericEvents[i] = createEvent(ep.epollEvents[i])
		}

		return ep.genericEvents[:n], nil
	}
}

func (ep *Epoll) Close() error {
	err := syscall.Close(ep.epfd)
	return err
}
