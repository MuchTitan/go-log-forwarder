package database

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

type DBManager struct {
	db *sql.DB
	mu sync.Mutex
}

// NewDBManager creates a new database manager instance
func NewDBManager(dbPath string) (*DBManager, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("cound not open sqlite3 database: %v", err)
	}

	logrus.WithField("file", dbPath).Debug("Opening Sqlite3 database.")

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	return &DBManager{
		db: db,
	}, nil
}

// ExecuteWrite performs a write operation safely
func (dm *DBManager) ExecuteWrite(query string, args ...any) (sql.Result, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	res, err := dm.db.Exec(query, args...)
	return res, err
}

// ExecuteWriteTx performs multiple write operations in a single transaction
func (dm *DBManager) ExecuteWriteTx(fn func(*sql.Tx) error) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	tx, err := dm.db.Begin()
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Query performs a read operation (Row)
func (dm *DBManager) QueryRow(query string, args ...any) *sql.Row {
	return dm.db.QueryRow(query, args...)
}

// Query performs a read operation (Rows)
func (dm *DBManager) Query(query string, args ...any) (*sql.Rows, error) {
	return dm.db.Query(query, args...)
}

// Close closes the database connection
func (dm *DBManager) Close() error {
	return dm.db.Close()
}