package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"sync"
)

type Config struct {
	ServerUrl  string `json:"serverUrl"`
	ServerPort int    `json:"serverPort"`
	DbFile     string `json:"dbFile"`
	DB         *sql.DB
}

var (
	cfg  *Config
	once sync.Once
)

func OpenDB(file string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", file)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS directories (
		id INTEGER NOT NULL PRIMARY KEY,
		time DATETIME NOT NULL,
		path TEXT);`); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS files (
		id INTEGER NOT NULL PRIMARY KEY,
		time DATETIME NOT NULL,
		path TEXT,
		LastLineNum INTEGER);`); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS lines (
		id INTEGER NOT NULL PRIMARY KEY,
		path TEXT,
		data TEXT,
		LineNum INTEGER);`); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return db, nil
}

func LoadConfig() *Config {
	once.Do(func() {
		file, err := os.Open("config.json")
		if err != nil {
			log.Fatalf("Failed to open config file: %v", err)
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		cfg = &Config{}
		err = decoder.Decode(cfg)
		if err != nil {
			log.Fatalf("Failed to decode config file: %v", err)
		}
		cfg.DB, err = OpenDB(cfg.DbFile)
		if err != nil {
			log.Fatalf("Cant open DB: %v", err)
		}
	})
	return cfg
}

func Get() *Config {
	if cfg == nil {
		LoadConfig()
	}
	return cfg
}
