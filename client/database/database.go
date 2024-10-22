package database

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func OpenDB(dbFile string) error {
	var err error
	DB, err = sql.Open("sqlite3", dbFile)
	if err != nil {
		return err
	}
	err = createRouterTable()
	if err != nil {
		return err
	}
	err = createRetryDataTable()
	if err != nil {
		return err
	}
	err = createTailFileStateTable()
	return err
}

func GetDB() *sql.DB {
	if DB == nil {
		log.Fatalln("Trying to get a DB that is not opened yet")
	}
	return DB
}

func CloseDB() {
	DB.Close()
}

func CleanUpRetryData() error {
	_, err := DB.Exec(`DELETE FROM retry_data where status = true`)
	return err
}

// Function to create tail_file_state table
func createTailFileStateTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS tail_file_state (
		filepath TEXT PRIMARY KEY,
		last_send_line INTEGER,
		checksum BLOB,
		inode_number INTEGER
	);`
	_, err := DB.Exec(query)
	if err != nil {
		return err
	}
	return nil
}

// Function to create router table
func createRouterTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS router (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		output TEXT NOT NULL,
		input TEXT NOT NULL
	);`
	_, err := DB.Exec(query)
	if err != nil {
		return err
	}
	return nil
}

// Function to create retry_data table
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
	if err != nil {
		return err
	}
	return nil
}
