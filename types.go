package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/thiagokokada/hyprland-go/event"
)

const (
	DebounceTime = 3 * time.Second
	DefaultGeneralDebounceTime = 500 * time.Millisecond
	DefaultIdleThreshold = 15 * time.Minute
	IdleSocketPath = "/tmp/hyprtracker-idle.sock"
)

func GetDefaultLogFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./hyprland_activity.log"
	}
	return filepath.Join(homeDir, ".local", "share", "hyprtracker", "hyprland_activity.log")
}

var DefaultLogFilePath = GetDefaultLogFilePath()

var TerminalEmulators = []string{
	"kitty",
	"alacritty",
	"terminology",
	"konsole",
	"gnome-terminal",
	"xfce4-terminal",
	"xterm",
	"urxvt",
	"termite",
	"st",
	"foot",
	"wezterm",
}

type LogEntry struct {
	Timestamp time.Time          `json:"timestamp"`
	EventType string             `json:"eventType"`
	EventData event.ActiveWindow `json:"eventData"`
	IsIdle    bool               `json:"isIdle,omitempty"`
}

type TimeSummary struct {
	Name     string
	Duration time.Duration
}

type LoggerConfig struct {
	TerminalDebounceTime   time.Duration
	GeneralDebounceTime    time.Duration
	UseExternalIdleManager bool
}

func IsTerminalEmulator(windowName string) bool {
	for _, terminal := range TerminalEmulators {
		if windowName == terminal {
			return true
		}
	}
	return false
}

func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
