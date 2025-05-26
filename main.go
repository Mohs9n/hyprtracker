package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	daemonFlag := flag.Bool("daemon", false, "Run as a daemon to collect window activity")
	logFileFlag := flag.String("logfile", DefaultLogFilePath, "Path to the log file")
	keywordsFlag := flag.String("keywords", "", "Comma-separated list of keywords to filter related activities (e.g., \"firefox,projectX,mydoc\")")
	minDurationFlag := flag.Int("min-duration", 0, "Minimum duration in seconds to include in the output (e.g., 1 will filter out activities less than 1 second)")
	appOnlyFlag := flag.Bool("app-only", false, "Only display per-application report, skip window details")
	
	// New configuration options
	terminalDebounceFlag := flag.Int("terminal-debounce", int(DebounceTime.Seconds()), "Terminal debounce time in seconds (default: 3)")
	generalDebounceFlag := flag.Int("general-debounce", int(DefaultGeneralDebounceTime.Seconds()), "General debounce time in seconds (default: 0.5)")
	
	// External idle manager integration
	useExternalIdleFlag := flag.Bool("use-external-idle", true, "Use external idle manager")
	idleSignalFlag := flag.String("idle-signal", "", "Send idle signal to running daemon: 'start' to mark idle start, 'end' to mark idle end")

	flag.Parse()

	if *idleSignalFlag != "" {
		if err := SendIdleSignal(*idleSignalFlag); err != nil {
			log.Fatalf("Error sending idle signal: %v", err)
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
			UseExternalIdleManager: *useExternalIdleFlag,
		}
		RunDaemonWithConfig(logFilePath, config)
	} else {
		minDuration := time.Duration(*minDurationFlag) * time.Second
		RunAnalysis(logFilePath, *keywordsFlag, minDuration, *appOnlyFlag)
	}
}
