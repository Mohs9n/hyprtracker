package main

import (
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

	// Skip event processing if tracking is paused
	if trackingPaused {
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

// This function has been removed in favor of the SQLite database implementation in database.go
