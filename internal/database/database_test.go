package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// isSQLiteAvailable checks if SQLite is actually available (requires CGO)
func isSQLiteAvailable() bool {
	tmpDir, err := os.MkdirTemp("", "sqlite-test")
	if err != nil {
		return false
	}
	defer os.RemoveAll(tmpDir)

	dbFile := filepath.Join(tmpDir, "test.db")
	dbManager, err := GetDBManager(dbFile)
	if err != nil {
		return false
	}
	defer dbManager.Close()

	// Try to actually execute a SQL statement
	_, err = dbManager.ExecuteWrite("CREATE TABLE test (id INTEGER)")
	if err != nil {
		return false
	}

	return true
}

// TestGetDBManager tests database manager creation
func TestGetDBManager(t *testing.T) {
	if !isSQLiteAvailable() {
		t.Skip("Skipping test: SQLite not available (CGO_ENABLED=0)")
	}

	// Create temp database
	tmpFile := t.TempDir() + "/test.db"

	dbManager, err := GetDBManager(tmpFile)
	if err != nil {
		t.Fatalf("GetDBManager() failed: %v", err)
	}

	if dbManager == nil {
		t.Fatal("GetDBManager() returned nil")
	}

	if dbManager.db == nil {
		t.Error("Database connection not initialized")
	}

	// Cleanup
	dbManager.Close()
}

// TestExecuteWrite tests write operations
func TestExecuteWrite(t *testing.T) {
	if !isSQLiteAvailable() {
		t.Skip("Skipping test: SQLite not available (CGO_ENABLED=0)")
	}

	tmpFile := t.TempDir() + "/test.db"
	dbManager, err := GetDBManager(tmpFile)
	if err != nil {
		t.Fatalf("GetDBManager() failed: %v", err)
	}
	defer dbManager.Close()

	// Create table
	_, err = dbManager.ExecuteWrite("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("ExecuteWrite() failed: %v", err)
	}

	// Insert data
	result, err := dbManager.ExecuteWrite("INSERT INTO test (name) VALUES (?)", "test_value")
	if err != nil {
		t.Fatalf("ExecuteWrite() insert failed: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected != 1 {
		t.Errorf("Expected 1 row affected, got %d", rowsAffected)
	}
}

// TestExecuteWriteTx tests transaction operations
func TestExecuteWriteTx(t *testing.T) {
	if !isSQLiteAvailable() {
		t.Skip("Skipping test: SQLite not available (CGO_ENABLED=0)")
	}

	tmpFile := t.TempDir() + "/test.db"
	dbManager, err := GetDBManager(tmpFile)
	if err != nil {
		t.Fatalf("GetDBManager() failed: %v", err)
	}
	defer dbManager.Close()

	// Create table
	dbManager.ExecuteWrite("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")

	// Execute transaction
	err = dbManager.ExecuteWriteTx(func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO test (name) VALUES (?)", "value1")
		if err != nil {
			return err
		}
		_, err = tx.Exec("INSERT INTO test (name) VALUES (?)", "value2")
		return err
	})

	if err != nil {
		t.Fatalf("ExecuteWriteTx() failed: %v", err)
	}

	// Verify data
	rows, err := dbManager.Query("SELECT COUNT(*) FROM test")
	if err != nil {
		t.Fatalf("Query() failed: %v", err)
	}
	defer rows.Close()

	var count int
	if rows.Next() {
		rows.Scan(&count)
	}

	if count != 2 {
		t.Errorf("Expected 2 rows, got %d", count)
	}
}

// TestExecuteWriteTx_Rollback tests transaction rollback
func TestExecuteWriteTx_Rollback(t *testing.T) {
	if !isSQLiteAvailable() {
		t.Skip("Skipping test: SQLite not available (CGO_ENABLED=0)")
	}

	tmpFile := t.TempDir() + "/test.db"
	dbManager, err := GetDBManager(tmpFile)
	if err != nil {
		t.Fatalf("GetDBManager() failed: %v", err)
	}
	defer dbManager.Close()

	// Create table
	dbManager.ExecuteWrite("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")

	// Execute failing transaction
	err = dbManager.ExecuteWriteTx(func(tx *sql.Tx) error {
		tx.Exec("INSERT INTO test (name) VALUES (?)", "value1")
		// Force error
		return sql.ErrTxDone
	})

	if err == nil {
		t.Error("Expected error from transaction, got nil")
	}

	// Verify rollback - no data should exist
	rows, _ := dbManager.Query("SELECT COUNT(*) FROM test")
	defer rows.Close()

	var count int
	if rows.Next() {
		rows.Scan(&count)
	}

	if count != 0 {
		t.Errorf("Expected 0 rows after rollback, got %d", count)
	}
}

// TestQueryRow tests single row queries
func TestQueryRow(t *testing.T) {
	if !isSQLiteAvailable() {
		t.Skip("Skipping test: SQLite not available (CGO_ENABLED=0)")
	}

	tmpFile := t.TempDir() + "/test.db"
	dbManager, err := GetDBManager(tmpFile)
	if err != nil {
		t.Fatalf("GetDBManager() failed: %v", err)
	}
	defer dbManager.Close()

	// Setup
	dbManager.ExecuteWrite("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	dbManager.ExecuteWrite("INSERT INTO test (name) VALUES (?)", "test_value")

	// Query
	row := dbManager.QueryRow("SELECT name FROM test WHERE id = ?", 1)

	var name string
	err = row.Scan(&name)
	if err != nil {
		t.Fatalf("QueryRow() scan failed: %v", err)
	}

	if name != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", name)
	}
}

// TestQuery tests multiple row queries
func TestQuery(t *testing.T) {
	if !isSQLiteAvailable() {
		t.Skip("Skipping test: SQLite not available (CGO_ENABLED=0)")
	}

	tmpFile := t.TempDir() + "/test.db"
	dbManager, err := GetDBManager(tmpFile)
	if err != nil {
		t.Fatalf("GetDBManager() failed: %v", err)
	}
	defer dbManager.Close()

	// Setup
	dbManager.ExecuteWrite("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	dbManager.ExecuteWrite("INSERT INTO test (name) VALUES (?)", "value1")
	dbManager.ExecuteWrite("INSERT INTO test (name) VALUES (?)", "value2")
	dbManager.ExecuteWrite("INSERT INTO test (name) VALUES (?)", "value3")

	// Query
	rows, err := dbManager.Query("SELECT name FROM test")
	if err != nil {
		t.Fatalf("Query() failed: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}

	if count != 3 {
		t.Errorf("Expected 3 rows, got %d", count)
	}
}

// TestClose tests database closure
func TestClose(t *testing.T) {
	if !isSQLiteAvailable() {
		t.Skip("Skipping test: SQLite not available (CGO_ENABLED=0)")
	}

	tmpFile := t.TempDir() + "/test.db"
	dbManager, err := GetDBManager(tmpFile)
	if err != nil {
		t.Fatalf("GetDBManager() failed: %v", err)
	}

	err = dbManager.Close()
	if err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}
