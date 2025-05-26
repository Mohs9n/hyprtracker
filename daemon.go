package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/thiagokokada/hyprland-go/event"
)

func RunDaemonWithConfig(logFilePath string, config LoggerConfig) {
	log.Printf("Starting Hyprland activity logger with configuration:")
	log.Printf("- Terminal Debounce Time: %s", FormatDuration(config.TerminalDebounceTime))
	log.Printf("- General Debounce Time: %s", FormatDuration(config.GeneralDebounceTime))
	if config.UseExternalIdleManager {
		log.Printf("- Using external idle manager: Yes (listening on %s)", IdleSocketPath)
	} else {
		log.Printf("- Using external idle manager: No (using internal idle detection)")
	}
	log.Printf("- Log file: %s", logFilePath)

	logEntryChan := make(chan LogEntry, 100)
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received, cleaning up...")
		cancel()
	}()

	wg.Add(1)
	go RunLogger(ctx, logEntryChan, logFilePath, &wg)

	// Start idle socket listener if using external idle manager
	if config.UseExternalIdleManager {
		if err := StartIdleSocketListener(ctx, &wg, logEntryChan); err != nil {
			log.Printf("Warning: Failed to start idle socket listener: %v", err)
			log.Println("Falling back to internal idle detection")
			config.UseExternalIdleManager = false
		}
	}

	client := event.MustClient()
	defer func() {
		log.Println("Closing Hyprland event client...")
		if err := client.Close(); err != nil {
			log.Printf("Error closing client: %v", err)
		}
	}()

	handler := NewDebouncedActivityLogger(logEntryChan, config)


	log.Printf("Subscribing to event: %s", event.EventActiveWindow)
	err := client.Subscribe(ctx, handler, event.EventActiveWindow)
	if err != nil {
		if ctx.Err() == nil {
			log.Fatalf("Failed to subscribe to Hyprland events: %v", err)
		} else {
			log.Printf("Subscription ended due to context cancellation: %v", err)
		}
	}

	<-ctx.Done()

	log.Println("Main event loop finished. Closing log channel...")
	close(logEntryChan)

	log.Println("Waiting for logger to finish...")
	wg.Wait()

	log.Println("Hyprland activity logger finished.")
}
