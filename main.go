package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"fyne.io/systray"

	_ "embed"
)

func main() {
	daemonFlag := flag.Bool("daemon", false, "Run as a daemon to collect window activity")
	logFileFlag := flag.String("logfile", DefaultLogFilePath, "Path to the log file")
	keywordsFlag := flag.String("keywords", "", "Comma-separated list of keywords to filter related activities (e.g., \"firefox,projectX,mydoc\")")
	minDurationFlag := flag.Int("min-duration", 0, "Minimum duration in seconds to include in the output (e.g., 1 will filter out activities less than 1 second)")
	appOnlyFlag := flag.Bool("app-only", true, "Only display per-application report, skip window details (default true)")
	
	// New configuration options
	terminalDebounceFlag := flag.Int("terminal-debounce", int(DebounceTime.Seconds()), "Terminal debounce time in seconds (default: 3)")
	generalDebounceFlag := flag.Int("general-debounce", int(DefaultGeneralDebounceTime.Seconds()), "General debounce time in seconds (default: 0.5)")
	
	// External idle manager integration
	idleSignalFlag := flag.String("idle-signal", "", "Send idle signal to running daemon: 'start' to mark idle start, 'end' to mark idle end")
	
	// Systray flag to make systray optional
	systrayFlag := flag.Bool("systray", false, "Enable system tray icon for controlling the daemon")
	
	// Flag to toggle pause via command line
	togglePauseFlag := flag.Bool("toggle-pause", false, "Toggle pause/resume on a running daemon")

	flag.Parse()

	if *idleSignalFlag != "" {
		if err := SendIdleSignal(*idleSignalFlag); err != nil {
			log.Fatalf("Error sending idle signal: %v", err)
		}
		return
	}
	
	if *togglePauseFlag {
		if err := SendPauseToggleSignal(); err != nil {
			log.Fatalf("Error sending pause toggle signal: %v", err)
		}
		return
	}

	nonFlagArgs := flag.Args()
	logFilePath := *logFileFlag
	if len(nonFlagArgs) > 0 {
		logFilePath = nonFlagArgs[0]
	}

	logDir := filepath.Dir(logFilePath)
	if logDir != "." && logDir != "" {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.Fatalf("Failed to create log directory: %v", err)
		}
	}

	if *daemonFlag {
		config := LoggerConfig{
			TerminalDebounceTime:   time.Duration(*terminalDebounceFlag) * time.Second,
			GeneralDebounceTime:    time.Duration(*generalDebounceFlag) * time.Second,
			EnableSystray:          *systrayFlag,
		}
		RunDaemonWithConfig(logFilePath, config)
	} else {
		minDuration := time.Duration(*minDurationFlag) * time.Second
		RunAnalysis(logFilePath, *keywordsFlag, minDuration, *appOnlyFlag)
	}
}


//go:embed icon.png
var iconData []byte

// Global variables to control tracking state
var (
	trackingPaused bool
	pauseMenuItem  *systray.MenuItem
	quitAppChan    = make(chan struct{})
	systrayEnabled bool
)

func systrayOnReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("HyprTracker")
	systray.SetTooltip("Hyprland Activity Tracker")
	
	// Status section
	mStatus := systray.AddMenuItem("Status: Active", "Current tracking status")
	mStatus.Disable()
	
	pauseMenuItem = systray.AddMenuItem("Pause Tracking", "Pause activity tracking")
	
	systray.AddSeparator()
	
	// Quit option
	mQuit := systray.AddMenuItem("Quit", "Quit HyprTracker")

	// Handle menu item clicks in goroutines
	go func() {
		for range pauseMenuItem.ClickedCh {
			if trackingPaused {
				resumeTracking()
				mStatus.SetTitle("Status: Active")
			} else {
				pauseTracking()
				mStatus.SetTitle("Status: Paused")
			}
		}
	}()
	
	go func() {
		for range mQuit.ClickedCh {
			log.Println("Quit selected from systray menu")
			close(quitAppChan) // Signal the main daemon loop to exit
		}
	}()
}

func pauseTracking() {
	trackingPaused = true
	log.Println("Activity tracking paused")
	
	// Update systray if enabled
	if systrayEnabled {
		pauseMenuItem.SetTitle("Resume Tracking")
		pauseMenuItem.SetTooltip("Resume activity tracking")
		systray.SetTooltip("HyprTracker (Paused)")
	}
}

func resumeTracking() {
	trackingPaused = false
	log.Println("Activity tracking resumed")
	
	// Update systray if enabled
	if systrayEnabled {
		pauseMenuItem.SetTitle("Pause Tracking")
		pauseMenuItem.SetTooltip("Pause activity tracking")
		systray.SetTooltip("HyprTracker (Active)")
	}
}

func toggleTracking() {
	if trackingPaused {
		resumeTracking()
	} else {
		pauseTracking()
	}
}

func systrayOnExit() {
	log.Println("Cleaning up systray resources")
}