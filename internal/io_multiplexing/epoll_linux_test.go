package io_multiplexing

import (
	"testing"
	"time"
)

func TestEpollWakeup(t *testing.T) {
	mux, err := CreateIOMultiplexer()
	if err != nil {
		t.Fatal(err)
	}
	defer mux.Close()

	result := make(chan []Event, 1)
	errCh := make(chan error, 1)

	go func() {
		events, err := mux.Wait()
		if err != nil {
			errCh <- err
			return
		}
		result <- events
	}()

	// Give Wait time to enter EpollWait.
	time.Sleep(20 * time.Millisecond)

	if err := mux.Wakeup(); err != nil {
		t.Fatal(err)
	}

	select {
	case events := <-result:
		if len(events) != 1 || !events[0].IsWakeup {
			t.Fatalf("unexpected events: %+v", events)
		}

	case err := <-errCh:
		t.Fatal(err)

	case <-time.After(2 * time.Second):
		t.Fatal("epoll was not woken up")
	}
}
