package database

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

var (
	db   *sql.DB
	once sync.Once
)

func Open(dbName string) error {
	var err error
	once.Do(func() {
		db, err = sql.Open("sqlite3", dbName)
	})
	return err
}

func Close() error {
	return db.Close()
}

func ExecDBQuery(query string, args ...any) error {
	if db == nil {
		return fmt.Errorf("No DB opened yet")
	}
	_, err := db.Exec(query, args...)
	if err != nil {
		return err
	}
	return nil
}
