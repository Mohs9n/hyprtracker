package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"fyne.io/systray"
	"github.com/thiagokokada/hyprland-go/event"
)

func RunDaemonWithConfig(logFilePath string, config LoggerConfig) {
	log.Printf("Starting Hyprland activity logger with configuration:")
	log.Printf("- Terminal Debounce Time: %s", FormatDuration(config.TerminalDebounceTime))
	log.Printf("- General Debounce Time: %s", FormatDuration(config.GeneralDebounceTime))

	// Optional systray
	var trayEnd func()
	if config.EnableSystray {
		log.Printf("- System Tray: Enabled")
		systrayEnabled = true
		var trayStart func()
		trayStart, trayEnd = systray.RunWithExternalLoop(systrayOnReady, systrayOnExit)
		trayStart()
		defer func() {
			if trayEnd != nil {
				trayEnd()
			}
		}()
	} else {
		log.Printf("- System Tray: Disabled")
		systrayEnabled = false
	}

	log.Printf("- Log file: %s", logFilePath)

	logEntryChan := make(chan LogEntry, 100)
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Handle both system signals and tray quit button if systray is enabled
	go func() {
		if config.EnableSystray {
			select {
			case <-sigChan:
				log.Println("Shutdown signal received, cleaning up...")
				cancel()
			case <-quitAppChan:
				log.Println("Quit requested from tray menu, cleaning up...")
				cancel()
			}
		} else {
			// If systray is disabled, only listen for system signals
			<-sigChan
			log.Println("Shutdown signal received, cleaning up...")
			cancel()
		}
	}()

	wg.Add(1)
	go RunLogger(ctx, logEntryChan, logFilePath, &wg)
	// Start socket listener for external commands (idle signals, pause toggle)
	if err := StartSocketListener(ctx, &wg, logEntryChan); err != nil {
		log.Printf("Warning: Failed to start socket listener: %v", err)
		log.Println("External control via command line will be unavailable")
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
