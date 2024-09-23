package main

import (
	"context"
	"fmt"
	"io"
	"log-forwarder-client/config"
	"log-forwarder-client/directory"
	"log-forwarder-client/utils"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.etcd.io/bbolt"
)

type LogOut interface {
	io.Writer
}

var (
	runningDirectorys []*directory.DirectoryState
	wg                *sync.WaitGroup
	parentCtx         context.Context
	cfg               *config.Config
	logger            *slog.Logger
	db                *bbolt.DB
)

func setupLogger() *os.File {
	// Open log file
	logFile, err := os.OpenFile("application.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	var logOut LogOut = utils.NewMultiWriter(os.Stdout, logFile)
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	logger = slog.New(slog.NewJSONHandler(logOut, opts))
	return logFile
}

func startNewDirectory(path string, parentCtx context.Context) *directory.DirectoryState {
	serverUrl := fmt.Sprintf("http://%s:%d", cfg.ServerUrl, cfg.ServerPort)
	dir := directory.NewDirectoryState(path, serverUrl, logger, wg, parentCtx)

	// Load state from database
	if err := dir.LoadState(db); err != nil {
		logger.Error("Failed to load state from database", "error", err)
		os.Exit(1)
	}

	go dir.Watch()
	runningDirectorys = append(runningDirectorys, dir)
	return dir
}

func openDB() {
	var err error
	db, err = bbolt.Open(cfg.DbFile, 0600, nil)
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
}

func saveToDB() {
	for _, dir := range runningDirectorys {
		if err := dir.SaveState(db); err != nil {
			logger.Error("Failed to save state to database", "error", err)
		}
	}
}

func main() {
	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg = &sync.WaitGroup{}

	// start logger
	logFile := setupLogger()
	defer logFile.Close()

	// Get configuration
	cfg = config.Get()
	logger.Info("Starting Log forwarder")

	// Open BBolt database
	openDB()
	defer db.Close()

	startNewDirectory("./test/*.log", parentCtx)
	startNewDirectory("/var/log/*", parentCtx)

	// Periodically save state (every 3 minutes)
	go func() {
		ticker := time.NewTicker(3 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-parentCtx.Done():
				return
			case <-ticker.C:
				saveToDB()
			}
		}
	}()

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("Log forwarder shutdown started")

	cancel()

	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		logger.Info("All goroutines completed successfully")
	case <-time.After(120 * time.Second):
		logger.Warn("Shutdown timed out, some goroutines may not have completed")
	}

	saveToDB()

	logger.Info("Log forwarder shutdown complete")
}
