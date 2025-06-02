package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type IdleMessage struct {
	Action string    // "start" or "end"
	Time   time.Time
}

// creates a Unix domain socket to listen for commands (idle events, pause toggle)
func StartSocketListener(ctx context.Context, wg *sync.WaitGroup, logChan chan<- LogEntry) error {
	if _, err := os.Stat(SocketPath); err == nil {
		if err := os.Remove(SocketPath); err != nil {
			return fmt.Errorf("failed to remove existing socket: %v", err)
		}
	}

	listener, err := net.Listen("unix", SocketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %v", err)
	}

	if err := os.Chmod(SocketPath, 0666); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %v", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer listener.Close()
		defer os.Remove(SocketPath)

		log.Printf("Command socket listener started at %s", SocketPath)

		connChan := make(chan net.Conn)
		errChan := make(chan error)

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					errChan <- err
					return
				}
				connChan <- conn
			}
		}()

		for {
			select {
			case <-ctx.Done():
				log.Println("Command socket listener shutting down")
				return
			case err := <-errChan:
				if !errors.Is(err, net.ErrClosed) {
					log.Printf("Error accepting connection: %v", err)
				}
				return
			case conn := <-connChan:
				go handleSocketConnection(conn, logChan)
			}
		}
	}()

	return nil
}

// processes a single connection to the command socket
func handleSocketConnection(conn net.Conn, logChan chan<- LogEntry) {
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		log.Printf("Error setting read deadline: %v", err)
		return
	}

	buf := make([]byte, 128)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("Error reading from socket: %v", err)
		return
	}

	message := strings.TrimSpace(string(buf[:n]))
	parts := strings.SplitN(message, " ", 2)
	command := parts[0]

	// Process commands
	switch command {
	case "idle":
		// Handle idle commands
		if len(parts) < 2 {
			log.Printf("Invalid idle command format: %s", message)
			_, _ = conn.Write([]byte("ERROR: Invalid command format"))
			return
		}
		
		idleParts := strings.SplitN(parts[1], " ", 2)
		action := idleParts[0]
		
		// Skip idle events processing if tracking is paused
		if trackingPaused {
			_, _ = conn.Write([]byte("OK"))
			return
		}
		
		var timestamp time.Time
		if len(idleParts) > 1 && idleParts[1] != "" {
			parsedTime, err := time.Parse(time.RFC3339, idleParts[1])
			if err != nil {
				log.Printf("Invalid timestamp format, using current time: %v", err)
				timestamp = time.Now()
			} else {
				timestamp = parsedTime
			}
		} else {
			timestamp = time.Now()
		}
		
		switch action {
		case "start":
			logChan <- LogEntry{
				Timestamp: timestamp,
				EventType: "idle_start",
				IsIdle:    true,
			}
			log.Printf("Received idle start signal at %s", timestamp.Format(time.RFC3339))
		case "end":
			logChan <- LogEntry{
				Timestamp: timestamp,
				EventType: "idle_end",
				IsIdle:    false,
			}
			log.Printf("Received idle end signal at %s", timestamp.Format(time.RFC3339))
		default:
			log.Printf("Unknown idle action: %s", action)
			_, _ = conn.Write([]byte("ERROR: Unknown idle action"))
			return
		}
		
	case "pause-toggle":
		// Toggle tracking state
		toggleTracking()
		log.Printf("Tracking state toggled via socket command")
		
	default:
		log.Printf("Unknown command: %s", command)
		_, _ = conn.Write([]byte("ERROR: Unknown command"))
		return
	}

	_, _ = conn.Write([]byte("OK"))
}

// sends a command to the daemon via the socket
func sendCommand(command string) error {
	if _, err := os.Stat(SocketPath); os.IsNotExist(err) {
		return fmt.Errorf("socket not found at %s - is the daemon running?", SocketPath)
	}

	conn, err := net.Dial("unix", SocketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to socket: %v", err)
	}
	defer conn.Close()

	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("error setting write deadline: %v", err)
	}

	if _, err := conn.Write([]byte(command)); err != nil {
		return fmt.Errorf("error sending command: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("error setting read deadline: %v", err)
	}

	buf := make([]byte, 128)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("error reading acknowledgment: %v", err)
	}

	response := string(buf[:n])
	if !strings.HasPrefix(response, "OK") {
		return fmt.Errorf("unexpected response from daemon: %s", response)
	}

	return nil
}

// sends an idle signal to the daemon
func SendIdleSignal(action string) error {
	if action != "start" && action != "end" {
		return fmt.Errorf("invalid idle action: %s (must be 'start' or 'end')", action)
	}

	command := fmt.Sprintf("idle %s %s", action, time.Now().Format(time.RFC3339))
	return sendCommand(command)
}

// sends a toggle-pause signal to the daemon
func SendPauseToggleSignal() error {
	return sendCommand("pause-toggle")
}


