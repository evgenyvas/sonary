// Package database
package database

import (
	"database/sql"
	"log"
	"sonary/internal/config"
	"sync"

	_ "modernc.org/sqlite"
)

var (
	instance *sql.DB
	once     sync.Once
)

type DBTX interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
}

func GetDB() *sql.DB {
	once.Do(func() {
		cfg := config.GetConfig()
		db, err := sql.Open("sqlite", cfg.DatabaseDsn)
		if err != nil {
			log.Fatal(err)
		}

		// Check if the database has already been initialized
		var tableName string
		err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='jobs'").Scan(&tableName)

		if err != nil {
			if err == sql.ErrNoRows {
				log.Println("Database is empty or uninitialized. Creating tables...")

				// Initialize the database schema
				_, err = db.Exec(`
					CREATE TABLE IF NOT EXISTS jobs (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						task_type TEXT NOT NULL,
						payload TEXT NOT NULL,         -- Stored as JSON string
						status TEXT DEFAULT 'pending', -- pending, running, completed, failed
						result TEXT,                   -- Stored as JSON string
						error_message TEXT,
						created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
						updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
					);

					CREATE TABLE artists (
						id INTEGER PRIMARY KEY,
						name TEXT NOT NULL UNIQUE
					);

					CREATE TABLE albums (
						id INTEGER PRIMARY KEY,
						artist_id INTEGER NOT NULL,

						title TEXT NOT NULL,
						year INTEGER,

						FOREIGN KEY (artist_id)
							REFERENCES artists(id),

						UNIQUE(artist_id, title)
					);

					CREATE TABLE directories (
						id INTEGER PRIMARY KEY,
						path TEXT UNIQUE,
						mtime INTEGER,
						last_scan INTEGER
					);

					CREATE TABLE tracks (
						id INTEGER PRIMARY KEY,
						album_id INTEGER NOT NULL,
						directory_id INTEGER NOT NULL,
						artist_id INTEGER NOT NULL,

						path TEXT NOT NULL,
						file_type TEXT NOT NULL,
						title TEXT NOT NULL,
						year INTEGER,
						genre TEXT,
						track_number INTEGER,
						duration INTEGER,
						lyrics TEXT,

						is_cue BOOLEAN NOT NULL DEFAULT 0,
						cue_file TEXT,
						cue_offset INTEGER,

						is_like BOOLEAN NOT NULL DEFAULT 0,

						FOREIGN KEY (album_id)
							REFERENCES albums(id),
						FOREIGN KEY (directory_id)
							REFERENCES directories(id),
						FOREIGN KEY (artist_id)
							REFERENCES artists(id)
					);
				`)
				if err != nil {
					log.Fatalf("Failed to create table: %v", err)
				}
				log.Println("Database initialized successfully.")
			} else {
				log.Fatalf("Database error: %v", err)
			}
		} else {
			log.Printf("Table '%s' already exists. Skipping initialization.\n", tableName)
		}

		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0)

		instance = db
	})
	return instance
}
