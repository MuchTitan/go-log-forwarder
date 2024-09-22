package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"log-forwarder-client/config"
	"log-forwarder-client/directory"
	"log-forwarder-client/utils"

	"go.etcd.io/bbolt"
)

type LogOut interface {
	io.Writer
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logFile, err := os.OpenFile("application.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	// Setup logger
	var logOut LogOut = utils.NewMultiWriter(os.Stdout, logFile)
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo, // Set the log level to Debug
	}
	logger := slog.New(slog.NewJSONHandler(logOut, opts))

	// Get configuration
	cfg := config.Get()
	serverUrl := fmt.Sprintf("http://%s:%d/test", cfg.ServerUrl, cfg.ServerPort)
	logger.Info("Starting application")

	// Open BBolt database
	db, err := bbolt.Open("state.db", 0600, nil)
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create DirectoryState
	wg := &sync.WaitGroup{}
	dir := directory.NewDirectoryState("./test/*.log", serverUrl, logger, wg, ctx)

	// Load state from database
	if err := dir.LoadState(db); err != nil {
		logger.Error("Failed to load state from database", "error", err)
		os.Exit(1)
	}

	// Start watching the directory
	go dir.Watch()

	// Periodically save state (every 3 minutes)
	go func() {
		ticker := time.NewTicker(3 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := dir.SaveState(db); err != nil {
					logger.Error("Failed to save state to database periodically", "error", err)
				}
			}
		}
	}()

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("Application shutdown started")

	cancel()

	// Create a channel to signal completion
	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		logger.Info("All goroutines completed successfully")
	case <-time.After(30 * time.Second):
		logger.Warn("Shutdown timed out, some goroutines may not have completed")
	}

	if err := dir.SaveState(db); err != nil {
		logger.Error("Failed to save final state to database", "error", err)
	}

	logger.Info("Application shutdown complete")
}
