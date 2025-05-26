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

// creates a Unix domain socket to listen for idle events
func StartIdleSocketListener(ctx context.Context, wg *sync.WaitGroup, logChan chan<- LogEntry) error {
	if _, err := os.Stat(IdleSocketPath); err == nil {
		if err := os.Remove(IdleSocketPath); err != nil {
			return fmt.Errorf("failed to remove existing socket: %v", err)
		}
	}

	listener, err := net.Listen("unix", IdleSocketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %v", err)
	}

	if err := os.Chmod(IdleSocketPath, 0666); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %v", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer listener.Close()
		defer os.Remove(IdleSocketPath)

		log.Printf("Idle socket listener started at %s", IdleSocketPath)

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
				log.Println("Idle socket listener shutting down")
				return
			case err := <-errChan:
				if !errors.Is(err, net.ErrClosed) {
					log.Printf("Error accepting connection: %v", err)
				}
				return
			case conn := <-connChan:
				go handleIdleConnection(conn, logChan)
			}
		}
	}()

	return nil
}

// processes a single connection to the idle socket
func handleIdleConnection(conn net.Conn, logChan chan<- LogEntry) {
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		log.Printf("Error setting read deadline: %v", err)
		return
	}

	buf := make([]byte, 128)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("Error reading from idle socket: %v", err)
		return
	}

	message := strings.TrimSpace(string(buf[:n]))
	parts := strings.SplitN(message, " ", 2)
	action := parts[0]

	var timestamp time.Time
	if len(parts) > 1 && parts[1] != "" {
		parsedTime, err := time.Parse(time.RFC3339, parts[1])
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
	}

	_, _ = conn.Write([]byte("OK"))
}

// sends an idle signal to the daemon
func SendIdleSignal(action string) error {
	if action != "start" && action != "end" {
		return fmt.Errorf("invalid idle action: %s (must be 'start' or 'end')", action)
	}

	if _, err := os.Stat(IdleSocketPath); os.IsNotExist(err) {
		return fmt.Errorf("idle socket not found at %s - is the daemon running?", IdleSocketPath)
	}

	conn, err := net.Dial("unix", IdleSocketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to idle socket: %v", err)
	}
	defer conn.Close()

	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("error setting write deadline: %v", err)
	}

	message := fmt.Sprintf("%s %s", action, time.Now().Format(time.RFC3339))
	if _, err := conn.Write([]byte(message)); err != nil {
		return fmt.Errorf("error sending idle signal: %v", err)
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
	if response != "OK" {
		return fmt.Errorf("unexpected response from daemon: %s", response)
	}

	return nil
}
