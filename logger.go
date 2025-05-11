package main

import (
	"context"
	"encoding/json"
	"github.com/thiagokokada/hyprland-go/event"
	"log"
	"os"
	"sync"
	"time"
)

type DebouncedActivityLogger struct {
	event.DefaultEventHandler
	logChan              chan<- LogEntry
	mu                   sync.Mutex
	lastWindow           string
	terminalDebounceInfo map[string]*TerminalDebounceInfo
}

type TerminalDebounceInfo struct {
	LastTitle string
	LastTime  time.Time
}

func (al *DebouncedActivityLogger) ActiveWindow(w event.ActiveWindow) {
	if w.Name == "" && w.Title == "" {
		return
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	if al.terminalDebounceInfo == nil {
		al.terminalDebounceInfo = make(map[string]*TerminalDebounceInfo)
	}

	windowKey := w.Name + "|" + w.Title

	if windowKey == al.lastWindow {
		return
	}

	isTerminal := IsTerminalEmulator(w.Name)

	if isTerminal {
		now := time.Now()

		termInfo, exists := al.terminalDebounceInfo[w.Name]
		if !exists {
			termInfo = &TerminalDebounceInfo{
				LastTitle: w.Title,
				LastTime:  now,
			}
			al.terminalDebounceInfo[w.Name] = termInfo
		} else if now.Sub(termInfo.LastTime) < DebounceTime {
			termInfo.LastTitle = w.Title
			termInfo.LastTime = now
			return
		} else {
			termInfo.LastTitle = w.Title
			termInfo.LastTime = now
		}
	}

	al.lastWindow = windowKey

	al.logChan <- LogEntry{
		Timestamp: time.Now(),
		EventType: string(event.EventActiveWindow),
		EventData: w,
	}
}

func RunLogger(ctx context.Context, logChan <-chan LogEntry, logFilePath string, wg *sync.WaitGroup) {
	defer wg.Done()

	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file %s: %v", logFilePath, err)
		return
	}
	defer func() {
		log.Println("Closing log file...")
		if err := file.Close(); err != nil {
			log.Printf("Error closing log file: %v", err)
		}
	}()

	log.Println("Logger goroutine started.")

	for {
		select {
		case entry, ok := <-logChan:
			if !ok {
				log.Println("Log channel closed, logger goroutine exiting.")
				return
			}
			writeEntryToFile(file, entry)

		case <-ctx.Done():
			log.Println("Context cancelled, logger goroutine flushing remaining entries...")
			flushRemainingEntries(logChan, file)
			return
		}
	}
}

func writeEntryToFile(file *os.File, entry LogEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	jsonData, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Error marshalling log entry to JSON: %v (Entry: %+v)", err, entry)
		return
	}
	if _, err := file.WriteString(string(jsonData) + "\n"); err != nil {
		log.Printf("Error writing to log file: %v", err)
	}
}

func flushRemainingEntries(logChan <-chan LogEntry, file *os.File) {
	for {
		select {
		case entry, ok := <-logChan:
			if !ok {
				return
			}
			writeEntryToFile(file, entry)
		default:
			return
		}
	}
}
