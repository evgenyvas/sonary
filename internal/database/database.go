// Package database
package database

import (
	"database/sql"
	"database/sql/driver"
	"log"
	"sonary/internal/config"
	"strings"
	"sync"

	"modernc.org/sqlite"
)

var (
	readerDB *sql.DB
	writerDB *sql.DB
	once     sync.Once
)

type DBTX interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
}

func sqliteDSN(path string) string {
	return "file:" + path + "?" +
		"_pragma=journal_mode(WAL)&" +
		"_pragma=synchronous(NORMAL)&" +
		"_pragma=busy_timeout(1000)&" +
		"_pragma=foreign_keys(ON)"
}

func initConnections() {
	err := sqlite.RegisterDeterministicScalarFunction("lower", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if len(args) == 0 || args[0] == nil {
				return nil, nil
			}
			str, ok := args[0].(string)
			if !ok {
				return args[0], nil
			}
			return strings.ToLower(str), nil
		})

	if err != nil {
		panic("Failed to register custom UTF-8 LOWER function: " + err.Error())
	}

	cfg := config.GetConfig()

	readerDB, err = sql.Open("sqlite", sqliteDSN(cfg.DatabaseDsn))
	if err != nil {
		log.Fatal(err)
	}
	if err = readerDB.Ping(); err != nil {
		log.Fatal(err)
	}

	readerDB.SetMaxOpenConns(10)
	readerDB.SetMaxIdleConns(10)
	readerDB.SetConnMaxLifetime(0)

	writerDB, err = sql.Open("sqlite", sqliteDSN(cfg.DatabaseDsn))
	if err != nil {
		log.Fatal(err)
	}
	if err = writerDB.Ping(); err != nil {
		log.Fatal(err)
	}

	writerDB.SetMaxOpenConns(1)
	writerDB.SetMaxIdleConns(1)
	writerDB.SetConnMaxLifetime(0)

	initDatabase(writerDB)
}

func initDatabase(db DBTX) {
	// Check if the database has already been initialized
	var tableName string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='jobs'").Scan(&tableName)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("Database is empty or uninitialized. Creating tables...")

			// Initialize the database schema
			_, err = db.Exec(`
				CREATE TABLE IF NOT EXISTS jobs (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					parent_id INTEGER,             -- Parent Job
					task_type TEXT NOT NULL,
					payload TEXT NOT NULL,         -- Stored as JSON string
					status TEXT DEFAULT 'pending', -- pending, running, completed, failed
					result TEXT,                   -- Stored as JSON string
					error_message TEXT,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (parent_id)
					REFERENCES jobs(id)
				);

				CREATE TABLE artists (
					id INTEGER PRIMARY KEY,
					name TEXT NOT NULL UNIQUE
				);

				CREATE TABLE artist_relations (
					artist_id INTEGER NOT NULL,
					related_artist_id INTEGER NOT NULL,

					PRIMARY KEY (artist_id, related_artist_id),

					FOREIGN KEY (artist_id)
						REFERENCES artists(id)
						ON DELETE CASCADE,

					FOREIGN KEY (related_artist_id)
						REFERENCES artists(id)
						ON DELETE CASCADE
				);

				CREATE TABLE albums (
					id INTEGER PRIMARY KEY,
					artist_id INTEGER NOT NULL,

					title TEXT NOT NULL,
					year INTEGER,

					FOREIGN KEY (artist_id)
						REFERENCES artists(id)
						ON DELETE CASCADE
				);

				CREATE TABLE directories (
					id INTEGER PRIMARY KEY,
					path TEXT UNIQUE,
					mtime INTEGER,
					last_scan INTEGER,
					side_exists BOOLEAN NOT NULL DEFAULT 0,
					side_mtime INTEGER NOT NULL DEFAULT 0
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
					has_pregap BOOLEAN NOT NULL DEFAULT 0,
					pregap_duration INTEGER,
					lyrics TEXT,

					is_cue BOOLEAN NOT NULL DEFAULT 0,
					cue_file TEXT,
					cue_offset INTEGER,

					is_like BOOLEAN NOT NULL DEFAULT 0,

					FOREIGN KEY (album_id)
						REFERENCES albums(id)
						ON DELETE CASCADE,

					FOREIGN KEY (directory_id)
						REFERENCES directories(id)
						ON DELETE CASCADE,

					FOREIGN KEY (artist_id)
						REFERENCES artists(id)
						ON DELETE CASCADE
				);

				CREATE TABLE images (
					id INTEGER PRIMARY KEY,

					directory_id INTEGER,
					track_id INTEGER,
					artist_id INTEGER,

					path TEXT NOT NULL,
					type INTEGER NOT NULL,
					format TEXT NOT NULL,
					sort_order INTEGER,
					width INTEGER,
					height INTEGER,
					size INTEGER,
					mtime INTEGER,

					FOREIGN KEY (directory_id)
						REFERENCES directories(id)
						ON DELETE CASCADE,

					FOREIGN KEY (track_id)
						REFERENCES tracks(id)
						ON DELETE CASCADE,

					FOREIGN KEY (artist_id)
						REFERENCES artists(id)
						ON DELETE CASCADE
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
}

// two connections for read and write to avoid conflicts

func Reader() *sql.DB {
	once.Do(initConnections)
	return readerDB
}

func Writer() *sql.DB {
	once.Do(initConnections)
	return writerDB
}
