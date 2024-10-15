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
