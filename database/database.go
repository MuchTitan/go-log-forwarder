package database

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func OpenDB(dbFile string) error {
	var err error
	DB, err = sql.Open("sqlite3", dbFile)
	if err != nil {
		return err
	}

	// Create tables
	if err := createRouterTable(); err != nil {
		return err
	}
	if err := createRetryDataTable(); err != nil {
		return err
	}
	if err := createTailFileStateTable(); err != nil {
		return err
	}

	return nil
}

// GetDB returns the active database connection. Logs a fatal error if the database is not opened yet.
func GetDB() *sql.DB {
	if DB == nil {
		log.Fatalln("Trying to get a DB that is not opened yet")
	}
	return DB
}

// CloseDB closes the database connection.
func CloseDB() {
	if err := DB.Close(); err != nil {
		log.Printf("Failed to close database: %v", err)
	}
}

// CleanUpRetryData removes retry data with a true status from the retry_data table.
func CleanUpRetryData() error {
	_, err := DB.Exec(`DELETE FROM retry_data WHERE status = true`)
	return err
}

// createTailFileStateTable creates the tail_file_state table if it doesn't already exist.
func createTailFileStateTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS tail_file_state (
		filepath TEXT,
		seek_offset INTEGER,
		last_send_line INTEGER,
        checksum BLOB,
		inode_number INTEGER,
		PRIMARY KEY (filepath, inode_number,checksum)
	);`
	_, err := DB.Exec(query)
	return err
}

// createRouterTable creates the router table if it doesn't already exist.
func createRouterTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS router (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		output TEXT NOT NULL,
		input TEXT NOT NULL
	);`
	_, err := DB.Exec(query)
	return err
}

// createRetryDataTable creates the retry_data table if it doesn't already exist.
func createRetryDataTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS retry_data (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		data BLOB NOT NULL,
		outputs TEXT NOT NULL,
		status BOOLEAN DEFAULT 0,
		router_id INTEGER,
		FOREIGN KEY(router_id) REFERENCES router(id),
		UNIQUE(data, router_id)
	);`
	_, err := DB.Exec(query)
	return err
}

// SetupTestDB sets up a temporary SQLite3 database for testing, with cleanup on test completion.
func SetupTestDB(t *testing.T) *sql.DB {
	// Create a temporary directory for the test database
	tmpDir, err := os.MkdirTemp("", "test-db-*")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}

	// Create a temporary database file
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open the database
	if err := OpenDB(dbPath); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to open database: %v", err)
	}

	// Register cleanup function
	t.Cleanup(func() {
		CloseDB()
		os.RemoveAll(tmpDir)
	})

	// Clear any existing data
	tables := []string{"tail_file_state", "router", "retry_data"}
	for _, table := range tables {
		if _, err := DB.Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("Failed to clean table %s: %v", table, err)
		}
	}

	return DB
}
