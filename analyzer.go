package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/thiagokokada/hyprland-go/event"
)

func RunAnalysis(dbPath string, keywordsStr string, minDuration time.Duration, appOnly bool, timeRange string) {
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

	if len(relatedKeywords) > 0 {
		log.Printf("Filtering for related activities with keywords: [%s]", strings.Join(relatedKeywords, ", "))
	}
	if minDuration > 0 {
		log.Printf("Filtering out activities shorter than %s", FormatDuration(minDuration))
	}
	
	log.Printf("Using database at %s", dbPath)
	db, err := OpenDatabase(dbPath)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()
	
	// Calculate the time range based on the user's selection
	endTime := time.Now()
	var startTime time.Time
	
	switch timeRange {
	case "day":
		startTime = endTime.AddDate(0, 0, -1)
		fmt.Println("Analyzing data from the last 24 hours")
	case "week":
		startTime = endTime.AddDate(0, 0, -7)
		fmt.Println("Analyzing data from the last 7 days")
	case "month":
		startTime = endTime.AddDate(0, -1, 0)
		fmt.Println("Analyzing data from the last 30 days")
	case "year":
		startTime = endTime.AddDate(-1, 0, 0)
		fmt.Println("Analyzing data from the last year")
	case "all":
		startTime = time.Time{}
		fmt.Println("Analyzing all available data")
	default:
		startTime = endTime.AddDate(0, -1, 0) // Default to one month
		fmt.Println("Analyzing data from the last 30 days")
	}
	
	generateSummaryReport(db, startTime, endTime, relatedKeywords, minDuration, appOnly)
}

func generateSummaryReport(db *Database, startTime, endTime time.Time, relatedKeywords []string, minDuration time.Duration, appOnly bool) {
	var appDurations map[string]time.Duration
	var windowDurations map[string]time.Duration
	var totalKeywordMatchDuration time.Duration
	
	if len(relatedKeywords) > 0 {
		// Use the new database query for keyword filtering
		summaries, err := db.GetKeywordFilteredSummary(startTime, endTime, relatedKeywords)
		if err != nil {
			log.Fatalf("Error retrieving keyword-filtered summary: %v", err)
		}
		
		// Convert to map for compatibility with existing code
		appDurations = make(map[string]time.Duration)
		for _, summary := range summaries {
			appDurations[summary.Name] = summary.Duration
			totalKeywordMatchDuration += summary.Duration
		}
		
		// Get window-level details if needed
		if !appOnly {
			entries, err := db.GetEvents(startTime, endTime)
			if err != nil {
				log.Fatalf("Error retrieving events from database: %v", err)
			}
			
			_, windowDurations, _ = CalculateDurations(entries, relatedKeywords)
		}
	} else {
		// Use the optimized database query for application summary
		summaries, err := db.GetApplicationSummary(startTime, endTime)
		if err != nil {
			log.Fatalf("Error retrieving application summary: %v", err)
		}
		
		// Convert to map for compatibility with existing code
		appDurations = make(map[string]time.Duration)
		for _, summary := range summaries {
			appDurations[summary.Name] = summary.Duration
		}
		
		// Get window-level details if needed
		if !appOnly {
			entries, err := db.GetEvents(startTime, endTime)
			if err != nil {
				log.Fatalf("Error retrieving events from database: %v", err)
			}
			
			_, windowDurations, _ = CalculateDurations(entries, relatedKeywords)
		}
	}

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

func CalculateDurations(entries []LogEntry, relatedKeywords []string) (
	appDurations map[string]time.Duration,
	windowDurations map[string]time.Duration,
	totalKeywordMatchDuration time.Duration,
) {
	appDurations = make(map[string]time.Duration)
	windowDurations = make(map[string]time.Duration)

	inIdlePeriod := false
	
	for i := 0; i < len(entries)-1; i++ {
		current := entries[i]
		next := entries[i+1]

		// Skip if timestamps are out of order
		if !next.Timestamp.After(current.Timestamp) {
			log.Printf("Warning: Found out-of-order or duplicate timestamp at entry %d and %d. Skipping duration calculation for this interval.", i, i+1)
			continue
		}
		
		// Handle idle events
		if current.EventType == "idle_start" {
			inIdlePeriod = true
			continue
		}
		
		if current.EventType == "idle_end" {
			inIdlePeriod = false
			continue
		}
		
		// Skip if we're in an idle period
		if inIdlePeriod {
			continue
		}
		
		// Skip if the next entry is an idle_start (we'll handle this duration separately)
		if next.EventType == "idle_start" {
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
			
			continue
		}
		
		// Regular window change
		if current.EventType == string(event.EventActiveWindow) && next.EventType == string(event.EventActiveWindow) {
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
