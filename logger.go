package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/thiagokokada/hyprland-go/event"
)

type DebouncedActivityLogger struct {
	event.DefaultEventHandler
	logChan              chan<- LogEntry
	mu                   sync.Mutex
	lastWindow           string
	terminalDebounceInfo map[string]*TerminalDebounceInfo
	lastActivityTime     time.Time
	isIdle               bool
	config               LoggerConfig
}

type TerminalDebounceInfo struct {
	LastTitle string
	LastTime  time.Time
}

func NewDebouncedActivityLogger(logChan chan<- LogEntry, config LoggerConfig) *DebouncedActivityLogger {
	return &DebouncedActivityLogger{
		logChan:          logChan,
		lastActivityTime: time.Now(),
		isIdle:           false,
		config:           config,
	}
}

func (al *DebouncedActivityLogger) ActiveWindow(w event.ActiveWindow) {
	if w.Name == "" && w.Title == "" {
		return
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	now := time.Now()

	al.lastActivityTime = now

	if al.terminalDebounceInfo == nil {
		al.terminalDebounceInfo = make(map[string]*TerminalDebounceInfo)
	}

	windowKey := w.Name + "|" + w.Title

	if windowKey == al.lastWindow {
		return
	}

	// General debounce
	if al.config.GeneralDebounceTime > 0 && al.lastWindow != "" {
		// If the window change happens too quickly, we might skip logging it
		lastLogTime, lastLogExists := al.getLastLogTimeForKey(al.lastWindow)
		if lastLogExists && now.Sub(lastLogTime) < al.config.GeneralDebounceTime {
			// Update the last window but don't log
			al.lastWindow = windowKey
			return
		}
	}

	isTerminal := IsTerminalEmulator(w.Name)

	if isTerminal {
		termInfo, exists := al.terminalDebounceInfo[w.Name]
		if !exists {
			termInfo = &TerminalDebounceInfo{
				LastTitle: w.Title,
				LastTime:  now,
			}
			al.terminalDebounceInfo[w.Name] = termInfo
		} else if now.Sub(termInfo.LastTime) < al.config.TerminalDebounceTime {
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
		Timestamp: now,
		EventType: string(event.EventActiveWindow),
		EventData: w,
	}
}

func (al *DebouncedActivityLogger) getLastLogTimeForKey(windowKey string) (time.Time, bool) {
	if isTerminal := IsTerminalEmulator(windowKey[:strings.Index(windowKey, "|")]); isTerminal {
		terminalName := windowKey[:strings.Index(windowKey, "|")]
		if info, exists := al.terminalDebounceInfo[terminalName]; exists {
			return info.LastTime, true
		}
	}
	return time.Time{}, false
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
