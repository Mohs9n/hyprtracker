package main

import (
	"context"
	"github.com/thiagokokada/hyprland-go/event"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func RunDaemon(logFilePath string) {
	log.Printf("Starting Hyprland activity logger with terminal debouncing. Log file: %s", logFilePath)

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

	client := event.MustClient()
	defer func() {
		log.Println("Closing Hyprland event client...")
		if err := client.Close(); err != nil {
			log.Printf("Error closing client: %v", err)
		}
	}()

	handler := &DebouncedActivityLogger{
		logChan: logEntryChan,
	}

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
