package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"
)

func RunAnalysis(logFilePath string, keywordsStr string, minDuration time.Duration, appOnly bool) {
	var relatedKeywords []string
	if keywordsStr != "" {
		rawKeywords := strings.Split(keywordsStr, ",")
		for _, kw := range rawKeywords {
			trimmedKw := strings.TrimSpace(kw)
			if trimmedKw != "" {
				relatedKeywords = append(relatedKeywords, strings.ToLower(trimmedKw))
			}
		}
	}

	// log.Printf("Processing activity log: %s", logFilePath)
	if len(relatedKeywords) > 0 {
		log.Printf("Filtering for related activities with keywords: [%s]", strings.Join(relatedKeywords, ", "))
	}
	if minDuration > 0 {
		log.Printf("Filtering out activities shorter than %s", FormatDuration(minDuration))
	}

	file, err := os.Open(logFilePath)
	if err != nil {
		log.Fatalf("Error opening log file '%s': %v", logFilePath, err)
	}
	defer file.Close()

	entries, err := ParseLogEntries(file)
	if err != nil {
		log.Fatalf("Error parsing log entries: %v", err)
	}

	if len(entries) < 2 {
		log.Println("Not enough entries to calculate durations. Need at least 2.")
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	appDurations, windowDurations, totalKeywordMatchDuration := CalculateDurations(entries, relatedKeywords)

	if len(relatedKeywords) > 0 {
		fmt.Printf("\n--- Total Time For Activities Matching Keywords: [%s] ---\n", strings.Join(relatedKeywords, ", "))
		fmt.Printf("Total Duration: %s\n", FormatDuration(totalKeywordMatchDuration))

		fmt.Printf("\n--- Time Spent Per Application (Filtered by Keywords: [%s]) ---\n", strings.Join(relatedKeywords, ", "))
		PrintSortedSummary(appDurations, minDuration)
		
		if !appOnly {
			fmt.Printf("\n--- Time Spent Per Window (Filtered by Keywords: [%s]) ---\n", strings.Join(relatedKeywords, ", "))
			PrintSortedSummary(windowDurations, minDuration)
		}
	} else {
		fmt.Println("\n--- Time Spent Per Application ---")
		PrintSortedSummary(appDurations, minDuration)

		if !appOnly {
			fmt.Println("\n--- Time Spent Per Window (App - Title) ---")
			PrintSortedSummary(windowDurations, minDuration)
		}
	}
}

func ParseLogEntries(reader io.Reader) ([]LogEntry, error) {
	var entries []LogEntry
	scanner := bufio.NewScanner(reader)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			log.Printf("Warning: Skipping line %d due to unmarshal error: %v (Line: %s)", lineNumber, err, line)
			continue
		}

		if entry.EventType == "activewindow" {
			entries = append(entries, entry)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return entries, nil
}

func CalculateDurations(entries []LogEntry, relatedKeywords []string) (
	appDurations map[string]time.Duration,
	windowDurations map[string]time.Duration,
	totalKeywordMatchDuration time.Duration,
) {
	appDurations = make(map[string]time.Duration)
	windowDurations = make(map[string]time.Duration)

	for i := 0; i < len(entries)-1; i++ {
		current := entries[i]
		next := entries[i+1]

		if !next.Timestamp.After(current.Timestamp) {
			log.Printf("Warning: Found out-of-order or duplicate timestamp at entry %d and %d. Skipping duration calculation for this interval.", i, i+1)
			continue
		}

		duration := next.Timestamp.Sub(current.Timestamp)
		isMatch := false
		if len(relatedKeywords) > 0 {
			searchText := strings.ToLower(current.EventData.Name + " " + current.EventData.Title)
			for _, keyword := range relatedKeywords {
				if strings.Contains(searchText, keyword) {
					isMatch = true
					break
				}
			}
		}

		shouldProcessForSummaries := (len(relatedKeywords) == 0) || isMatch

		if shouldProcessForSummaries {
			appDurations[current.EventData.Name] += duration
			windowKey := fmt.Sprintf("%s - %s", current.EventData.Name, current.EventData.Title)
			windowDurations[windowKey] += duration
		}

		if len(relatedKeywords) > 0 && isMatch {
			totalKeywordMatchDuration += duration
		}
	}

	return appDurations, windowDurations, totalKeywordMatchDuration
}

func PrintSortedSummary(durations map[string]time.Duration, minDuration time.Duration) {
	if len(durations) == 0 {
		fmt.Println("No duration data to display for this filter.")
		return
	}

	summaryList := make([]TimeSummary, 0, len(durations))
	for name, duration := range durations {
		if duration >= minDuration {
			summaryList = append(summaryList, TimeSummary{Name: name, Duration: duration})
		}
	}

	if len(summaryList) == 0 {
		fmt.Printf("No activities lasted longer than %s.\n", FormatDuration(minDuration))
		return
	}

	sort.Slice(summaryList, func(i, j int) bool {
		return summaryList[i].Duration > summaryList[j].Duration
	})

	for _, item := range summaryList {
		fmt.Printf("%-60s : %s\n", item.Name, FormatDuration(item.Duration))
	}
}
