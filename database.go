package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/thiagokokada/hyprland-go/event"
)

const (
	schemaVersion   = 1
	createTablesSQL = `
		CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT
		);
		CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY,
			timestamp TEXT NOT NULL,
			event_type TEXT NOT NULL,
			window_name TEXT,
			window_title TEXT,
			is_idle BOOLEAN DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
		CREATE INDEX IF NOT EXISTS idx_events_window ON events(window_name);
	`
)

type Database struct {
	db         *sql.DB
	insertStmt *sql.Stmt
}

func GetDefaultDBPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./hyprtracker.db"
	}
	dbDir := filepath.Join(homeDir, ".local", "share", "hyprtracker")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Printf("Failed to create database directory: %v", err)
		return "./hyprtracker.db"
	}
	return filepath.Join(dbDir, "hyprtracker.db")
}

func OpenDatabase(dbPath string) (*Database, error) {
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %v", err)
	}

	// Create tables if they don't exist
	if _, err := db.Exec(createTablesSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	// Check/set schema version
	var version string
	err = db.QueryRow("SELECT value FROM meta WHERE key = 'schema_version'").Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			_, err = db.Exec("INSERT INTO meta (key, value) VALUES ('schema_version', ?)", fmt.Sprintf("%d", schemaVersion))
			if err != nil {
				db.Close()
				return nil, fmt.Errorf("failed to set schema version: %v", err)
			}
		} else {
			db.Close()
			return nil, fmt.Errorf("failed to check schema version: %v", err)
		}
	}

	stmt, err := db.Prepare(`
		INSERT INTO events (timestamp, event_type, window_name, window_title, is_idle) 
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to prepare insert statement: %v", err)
	}

	return &Database{
		db:         db,
		insertStmt: stmt,
	}, nil
}

func (d *Database) Close() error {
	if d.insertStmt != nil {
		if err := d.insertStmt.Close(); err != nil {
			log.Printf("Error closing prepared statement: %v", err)
		}
	}
	return d.db.Close()
}

func (d *Database) InsertLogEntry(entry LogEntry) error {
	_, err := d.insertStmt.Exec(
		entry.Timestamp.Format(time.RFC3339),
		entry.EventType,
		entry.EventData.Name,
		entry.EventData.Title,
		entry.IsIdle,
	)
	if err != nil {
		return fmt.Errorf("failed to insert log entry: %v", err)
	}
	return nil
}

func (d *Database) GetEvents(startTime, endTime time.Time) ([]LogEntry, error) {
	query := `
		SELECT timestamp, event_type, window_name, window_title, is_idle
		FROM events
		WHERE timestamp BETWEEN ? AND ?
		ORDER BY timestamp
	`
	rows, err := d.db.Query(query,
		startTime.Format(time.RFC3339),
		endTime.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var timestampStr string
		var eventType, windowName, windowTitle string
		var isIdle bool

		if err := rows.Scan(&timestampStr, &eventType, &windowName, &windowTitle, &isIdle); err != nil {
			return nil, fmt.Errorf("row scan failed: %v", err)
		}

		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			return nil, fmt.Errorf("timestamp parse failed: %v", err)
		}

		entries = append(entries, LogEntry{
			Timestamp: timestamp,
			EventType: eventType,
			EventData: event.ActiveWindow{
				Name:  windowName,
				Title: windowTitle,
			},
			IsIdle: isIdle,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration failed: %v", err)
	}

	return entries, nil
}

func (d *Database) GetApplicationSummary(startTime, endTime time.Time) ([]TimeSummary, error) {
	query := `
		WITH time_ranges AS (
			SELECT 
				window_name,
				timestamp AS start_time,
				LEAD(timestamp) OVER (ORDER BY timestamp) AS end_time
			FROM events
			WHERE timestamp BETWEEN ? AND ?
			ORDER BY timestamp
		)
		SELECT 
			window_name,
			SUM((julianday(end_time) - julianday(start_time)) * 86400) AS duration_seconds
		FROM time_ranges
		WHERE end_time IS NOT NULL
		GROUP BY window_name
		ORDER BY duration_seconds DESC
	`

	rows, err := d.db.Query(query,
		startTime.Format(time.RFC3339),
		endTime.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}
	defer rows.Close()

	var summaries []TimeSummary
	for rows.Next() {
		var appName string
		var durationSeconds float64

		if err := rows.Scan(&appName, &durationSeconds); err != nil {
			return nil, fmt.Errorf("row scan failed: %v", err)
		}

		summaries = append(summaries, TimeSummary{
			Name:     appName,
			Duration: time.Duration(durationSeconds * float64(time.Second)),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration failed: %v", err)
	}

	return summaries, nil
}

// GetKeywordFilteredSummary returns application usage filtered by keywords
func (d *Database) GetKeywordFilteredSummary(startTime, endTime time.Time, keywords []string) ([]TimeSummary, error) {
	query := `
		WITH time_ranges AS (
			SELECT 
				window_name,
				window_title,
				timestamp AS start_time,
				LEAD(timestamp) OVER (ORDER BY timestamp) AS end_time
			FROM events
			WHERE timestamp BETWEEN ? AND ?
			AND event_type = ?
			AND is_idle = 0
			ORDER BY timestamp
		)
		SELECT 
			window_name,
			SUM((julianday(end_time) - julianday(start_time)) * 86400) AS duration_seconds
		FROM time_ranges
		WHERE end_time IS NOT NULL
	`

	// If there are keywords, add filtering conditions
	if len(keywords) > 0 {
		conditions := make([]string, len(keywords))
		for i, keyword := range keywords {
			conditions[i] = fmt.Sprintf("LOWER(window_name) LIKE '%%%s%%' OR LOWER(window_title) LIKE '%%%s%%'", 
				strings.ToLower(keyword), strings.ToLower(keyword))
		}
		query += " AND (" + strings.Join(conditions, " OR ") + ")"
	}

	query += `
		GROUP BY window_name
		ORDER BY duration_seconds DESC
	`

	rows, err := d.db.Query(query,
		startTime.Format(time.RFC3339),
		endTime.Format(time.RFC3339),
		string(event.EventActiveWindow),
	)
	if err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}
	defer rows.Close()

	var summaries []TimeSummary
	for rows.Next() {
		var appName string
		var durationSeconds float64

		if err := rows.Scan(&appName, &durationSeconds); err != nil {
			return nil, fmt.Errorf("row scan failed: %v", err)
		}

		summaries = append(summaries, TimeSummary{
			Name:     appName,
			Duration: time.Duration(durationSeconds * float64(time.Second)),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration failed: %v", err)
	}

	return summaries, nil
}

func RunDBLogger(ctx context.Context, logChan <-chan LogEntry, dbPath string, wg *sync.WaitGroup) {
	defer wg.Done()

	db, err := OpenDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
		return
	}
	defer func() {
		log.Println("Closing database...")
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	log.Printf("SQLite Database Logger started with DB: %s", dbPath)

	tx, err := db.db.Begin()
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
		return
	}

	txStmt, err := tx.Prepare(`
		INSERT INTO events (timestamp, event_type, window_name, window_title, is_idle)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		log.Fatalf("Failed to prepare transaction statement: %v", err)
		tx.Rollback()
		return
	}

	insertCount := 0
	commitThreshold := 100
	lastCommit := time.Now()
	commitInterval := 5 * time.Second

	for {
		select {
		case entry, ok := <-logChan:
			if !ok {
				log.Println("Log channel closed, database logger exiting.")
				if insertCount > 0 {
					if err := tx.Commit(); err != nil {
						log.Printf("Error committing final transaction: %v", err)
						tx.Rollback()
					}
				}
				return
			}

			_, err := txStmt.Exec(
				entry.Timestamp.Format(time.RFC3339),
				entry.EventType,
				entry.EventData.Name,
				entry.EventData.Title,
				entry.IsIdle,
			)
			if err != nil {
				log.Printf("Error inserting entry into database: %v", err)
				continue
			}

			insertCount++

			// Commit if we've reached the threshold or time interval
			now := time.Now()
			if insertCount >= commitThreshold || now.Sub(lastCommit) >= commitInterval {
				if err := tx.Commit(); err != nil {
					log.Printf("Error committing transaction: %v", err)
					tx.Rollback()
				} else {
					tx, err = db.db.Begin()
					if err != nil {
						log.Fatalf("Failed to begin new transaction: %v", err)
						return
					}

					txStmt, err = tx.Prepare(`
						INSERT INTO events (timestamp, event_type, window_name, window_title, is_idle)
						VALUES (?, ?, ?, ?, ?)
					`)
					if err != nil {
						log.Fatalf("Failed to prepare new transaction statement: %v", err)
						tx.Rollback()
						return
					}

					insertCount = 0
					lastCommit = now
				}
			}

		case <-ctx.Done():
			log.Println("Context canceled, database logger flushing and exiting...")
			if insertCount > 0 {
				if err := tx.Commit(); err != nil {
					log.Printf("Error committing final transaction: %v", err)
					tx.Rollback()
				}
			}
			return
		}
	}
}
