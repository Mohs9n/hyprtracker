package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
)

func main() {
	daemonFlag := flag.Bool("daemon", false, "Run as a daemon to collect window activity")
	logFileFlag := flag.String("logfile", DefaultLogFilePath, "Path to the log file")

	keywordsFlag := flag.String("keywords", "", "Comma-separated list of keywords to filter related activities (e.g., \"firefox,projectX,mydoc\")")

	flag.Parse()

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
		RunDaemon(logFilePath)
	} else {
		RunAnalysis(logFilePath, *keywordsFlag)
	}
}
