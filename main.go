package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"redis_core/internal/command"
	"redis_core/internal/config"
	"redis_core/internal/io_multiplexing"
	"redis_core/internal/protocol"
)

func main() {
	// Start listening for incoming connections
	addr := net.JoinHostPort(config.Host, config.Port)
	tcpAddr, err := net.ResolveTCPAddr(config.Protocol, addr)
	if err != nil {
		log.Fatalf("Failed to resolve server address: %v", err)
	}

	// Create a non-blocking TCP socket
	var (
		serverFd int
		sa       syscall.Sockaddr
	)

	if ip4 := tcpAddr.IP.To4(); ip4 != nil {
		serverFd, err = syscall.Socket(
			syscall.AF_INET,
			syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC,
			0,
		)
		if err != nil {
			log.Fatalf("Failed to create IPv4 socket: %v", err)
		}

		var addr4 syscall.SockaddrInet4
		addr4.Port = tcpAddr.Port
		copy(addr4.Addr[:], ip4)
		sa = &addr4
	} else if ip6 := tcpAddr.IP.To16(); ip6 != nil {
		serverFd, err = syscall.Socket(
			syscall.AF_INET6,
			syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC,
			0,
		)
		if err != nil {
			log.Fatalf("Failed to create IPv6 socket: %v", err)
		}

		var addr6 syscall.SockaddrInet6
		addr6.Port = tcpAddr.Port
		copy(addr6.Addr[:], ip6)
		sa = &addr6
	} else {
		log.Fatalf("Invalid server IP address: %s", tcpAddr.IP)
	}

	// Allow the address to be reused after restart
	if err := syscall.SetsockoptInt(
		serverFd,
		syscall.SOL_SOCKET,
		syscall.SO_REUSEADDR,
		1,
	); err != nil {
		log.Fatalf("Failed to set SO_REUSEADDR: %v", err)
	}

	// Bind and listen
	if err := syscall.Bind(serverFd, sa); err != nil {
		log.Fatalf("Failed to bind socket: %v", err)
	}

	if err := syscall.Listen(serverFd, syscall.SOMAXCONN); err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Server is listening on %s", addr)
	log.Printf("Server file descriptor: %d", serverFd)

	// Create I/O multiplexer
	mux, err := io_multiplexing.CreateIOMultiplexer()
	if err != nil {
		log.Fatalf("Failed to create I/O multiplexer: %v", err)
	}
	defer mux.Close()

	// Register server file descriptor for read events
	err = mux.Monitor(io_multiplexing.Event{
		Fd: serverFd,
		Op: io_multiplexing.OpRead,
	})
	if err != nil {
		log.Fatalf("Failed to monitor server FD: %v", err)
	}
	log.Printf("Server FD %d registered for read events", serverFd)

	// Handle termination signals.
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signalCh)

	go func() {
		sig := <-signalCh

		log.Printf("Received signal %s, initiating graceful shutdown", sig)

		if err := mux.Wakeup(); err != nil {
			log.Printf("Failed to wake up event loop: %v", err)
		}
	}()

	// Wait for events and handle them
	clientReadBuffers := make(map[int][]byte)
	clientWriteBuffers := make(map[int][]byte)

	shuttingDown := false

	for {
		events, err := mux.Wait()
		if err != nil {
			log.Fatalf("Failed to wait for events: %v", err)
		}

		// Process wakeup events first to prioritize shutdown.
		for _, event := range events {
			if event.IsWakeup && !shuttingDown {
				log.Println("Event loop received wakeup event")

				shuttingDown = true

				// Stop monitoring the listening socket.
				if err := mux.Remove(serverFd); err != nil {
					log.Printf("Failed to remove server FD from epoll: %v", err)
				}

				// Close the listening socket.
				if err := syscall.Close(serverFd); err != nil {
					log.Printf("Failed to close server FD: %v", err)
				} else {
					log.Println("Stopped accepting new connections")
				}
			}
		}

		// Handle the remaining events
		for _, event := range events {
			if event.IsWakeup {
				continue
			}

			if event.Fd == serverFd {
				if shuttingDown {
					continue
				}
				for {
					connFd, _, err := syscall.Accept4(
						serverFd,
						syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC,
					)

					if err != nil {
						if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
							break
						}
						if err == syscall.EINTR {
							continue
						}

						log.Println("accept error:", err)
						break
					}

					if err := mux.Monitor(io_multiplexing.Event{
						Fd: connFd,
						Op: io_multiplexing.OpRead,
					}); err != nil {
						log.Println("failed to monitor conn fd:", err)
						_ = syscall.Close(connFd)
						continue
					}

					clientReadBuffers[connFd] = nil
					clientWriteBuffers[connFd] = nil
					log.Printf("Client connected: fd %d", connFd)
				}

				continue
			}
			if event.Op&io_multiplexing.OpRead != 0 {
				handleReadable(event.Fd, mux, clientReadBuffers, clientWriteBuffers)
			}

			if event.Op&io_multiplexing.OpWrite != 0 {
				// If handleReadable closed client
				if len(clientWriteBuffers[event.Fd]) > 0 {
					handleWritable(event.Fd, mux, clientReadBuffers, clientWriteBuffers)
				}
			}
		}
		// Exit the event loop after all clients have disconnected.
		if shuttingDown && len(clientReadBuffers) == 0 {
			log.Println("All clients closed. Shutting down server.")
			break
		}
	}

}

func closeClient(fd int, mux io_multiplexing.IOMultiplexer, clientReadBuffers map[int][]byte, clientWriteBuffers map[int][]byte) {
	// Remove the client FD from epoll.
	if err := mux.Remove(fd); err != nil {
		log.Printf("Failed to remove client fd %d from epoll: %v", fd, err)
	}

	// Remove client buffers.
	delete(clientReadBuffers, fd)
	delete(clientWriteBuffers, fd)

	// Close the client socket.
	if err := syscall.Close(fd); err != nil {
		log.Printf("Failed to close client fd %d: %v", fd, err)
	}

	log.Printf("Client closed: fd %d", fd)
}

func handleWritable(fd int, mux io_multiplexing.IOMultiplexer, clientReadBuffers map[int][]byte, clientWriteBuffers map[int][]byte) {
	data := clientWriteBuffers[fd]

	for len(data) > 0 {
		n, err := syscall.Write(fd, data)

		if err == syscall.EINTR {
			continue
		}

		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			// Retain the unwritten portion and wait for the socket to be ready for writing
			clientWriteBuffers[fd] = data

			if updateErr := mux.Update(io_multiplexing.Event{
				Fd: fd,
				Op: io_multiplexing.OpRead | io_multiplexing.OpWrite,
			}); updateErr != nil {
				log.Printf("Failed to enable write events for fd %d: %v", fd, updateErr)
				closeClient(fd, mux, clientReadBuffers, clientWriteBuffers)
			}
			return
		}

		if err != nil {
			log.Printf("Write error on fd %d: %v", fd, err)
			closeClient(fd, mux, clientReadBuffers, clientWriteBuffers)
			return
		}

		if n == 0 {
			// Unable to make progress; do not allow a continuous busy loop
			clientWriteBuffers[fd] = data

			if updateErr := mux.Update(io_multiplexing.Event{
				Fd: fd,
				Op: io_multiplexing.OpRead | io_multiplexing.OpWrite,
			}); updateErr != nil {
				log.Printf("Failed to enable write events for fd %d: %v", fd, updateErr)
				closeClient(fd, mux, clientReadBuffers, clientWriteBuffers)
			}
			return
		}

		data = data[n:]
	}

	// The entire response has been recorded
	delete(clientWriteBuffers, fd)

	// No more data to be written: monitoring reads only
	if err := mux.Update(io_multiplexing.Event{
		Fd: fd,
		Op: io_multiplexing.OpRead,
	}); err != nil {
		log.Printf("Failed to disable write events for fd %d: %v", fd, err)
		closeClient(fd, mux, clientReadBuffers, clientWriteBuffers)
	}
}

func handleReadable(fd int, mux io_multiplexing.IOMultiplexer, clientReadBuffers map[int][]byte, clientWriteBuffers map[int][]byte) {
	buf := make([]byte, 4096)

	// Read data from the non-blocking socket.
	for {
		n, err := syscall.Read(fd, buf)

		if err == syscall.EINTR {
			continue
		}

		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			return
		}

		if err != nil {
			log.Printf("Read error on fd %d: %v", fd, err)
			closeClient(fd, mux, clientReadBuffers, clientWriteBuffers)
			return
		}

		if n == 0 {
			closeClient(fd, mux, clientReadBuffers, clientWriteBuffers)
			return
		}

		// Append incoming bytes to this client's buffer.
		clientReadBuffers[fd] = append(clientReadBuffers[fd], buf[:n]...)

		// Process all complete requests and queue their responses.
		processBufferedRequests(fd, mux, clientReadBuffers, clientWriteBuffers)

		// Stop if the client was closed during request processing.
		if _, ok := clientReadBuffers[fd]; !ok {
			return
		}

		// Try to write queued responses.
		if len(clientWriteBuffers[fd]) > 0 {
			handleWritable(fd, mux, clientReadBuffers, clientWriteBuffers)
		}

		return
	}
}

func processBufferedRequests(fd int, mux io_multiplexing.IOMultiplexer, clientReadBuffers map[int][]byte, clientWriteBuffers map[int][]byte) {
	// Parse all complete RESP values in the buffer.
	for len(clientReadBuffers[fd]) > 0 {
		// Skip leading empty lines (CRLF).
		if clientReadBuffers[fd][0] == '\r' {
			if len(clientReadBuffers[fd]) == 1 {
				// Wait for the next byte to determine whether this is CRLF.
				return
			}

			if clientReadBuffers[fd][1] == '\n' {
				clientReadBuffers[fd] = clientReadBuffers[fd][2:]
				continue
			}
		}

		value, consumed, err := protocol.Parse(clientReadBuffers[fd])

		if err == protocol.ErrNeedMoreData {
			// Preserve incomplete data for the next read event.
			return
		}

		if err != nil {
			log.Printf("RESP parse error on fd %d: %v", fd, err)
			closeClient(fd, mux, clientReadBuffers, clientWriteBuffers)
			return
		}

		if consumed <= 0 || consumed > len(clientReadBuffers[fd]) {
			log.Printf("Invalid consumed count on fd %d", fd)
			closeClient(fd, mux, clientReadBuffers, clientWriteBuffers)
			return
		}

		// Convert RESP value into a command.
		cmd, err := command.ParseCommand(value)
		var response []byte

		if err != nil {
			response, err = protocol.Encode(
				protocol.RespError("ERR invalid command"),
			)
		} else {
			response, err = cmd.Handle()

			if err != nil {
				response, err = protocol.Encode(
					protocol.RespError("ERR command failed"),
				)
			}
		}

		if err != nil {
			log.Printf("Failed to encode response on fd %d: %v", fd, err)
			closeClient(fd, mux, clientReadBuffers, clientWriteBuffers)
			return
		}

		// Consume the successfully parsed request.
		clientReadBuffers[fd] = clientReadBuffers[fd][consumed:]

		// Queue the response without writing immediately.
		clientWriteBuffers[fd] = append(clientWriteBuffers[fd], response...)
	}
}
