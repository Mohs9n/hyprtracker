package main

import (
	"fmt"
	"time"

	"slices"

	"github.com/thiagokokada/hyprland-go/event"
)

const (
	DebounceTime            = 3 * time.Second
	DefaultGeneralDebounceTime = 500 * time.Millisecond
	DefaultIdleThreshold     = 15 * time.Minute
	SocketPath              = "/tmp/hyprtracker.sock"
)


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
	EnableSystray          bool
	DBPath                 string
}

func IsTerminalEmulator(windowName string) bool {
	return slices.Contains(TerminalEmulators, windowName)
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

var DefaultDBPath = GetDefaultDBPath()
