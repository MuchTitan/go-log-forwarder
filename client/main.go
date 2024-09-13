package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
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
	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logFile, err := os.OpenFile("application.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	cfg := config.Get()
	// Setup logger
	var logOut LogOut = utils.NewMultiWriter(logFile)
	opts := &slog.HandlerOptions{
		Level: slog.Level(cfg.GetLogLevel()), // Set the log level
	}
	logger := slog.New(slog.NewJSONHandler(logOut, opts))

	// Get configuration
	serverUrl := fmt.Sprintf("http://%s:%d/test", cfg.ServerUrl, cfg.ServerPort)
	logger.Info("Starting application")

	// Open BBolt database
	db, err := bbolt.Open("state.db", 0600, nil)
	if err != nil {
		logger.Error("Failed to open database", "error", err)
	}
	defer db.Close()
	// Create DirectoryState
	dir := directory.NewDirectoryState("./test/*.log", serverUrl, logger)

	// Load state from database
	if err := dir.LoadState(db, ctx); err != nil {
		logger.Error("Failed to load state from database", "error", err)
		os.Exit(1)
	}

	// Start watching the directory
	go func() {
		if err := dir.Watch(ctx); err != nil {
			logger.Error("Directory watching stopped unexpectedly", "error", err)
		}
	}()

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

	// Cancel the context to stop all operations
	cancel()

	dir.Stop()

	if err := dir.SaveState(db); err != nil {
		logger.Error("Failed to save final state to database", "error", err)
	}

	logger.Info("Application shutdown complete")
}
