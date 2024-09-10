package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log-forwarder-client/config"
	"log-forwarder-client/directory"
	"log-forwarder-client/utils"

	"github.com/sirupsen/logrus"
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

	// Setup logger
	var logOut LogOut = utils.NewMultiWriter(os.Stdout, logFile)
	logger := logrus.New()
	logger.SetOutput(logOut)
	logger.SetLevel(logrus.InfoLevel)

	// Get configuration
	cfg := config.Get()
	serverUrl := fmt.Sprintf("http://%s:%d/test", cfg.ServerUrl, cfg.ServerPort)

	// Open BBolt database
	db, err := bbolt.Open("state.db", 0600, nil)
	if err != nil {
		logger.WithError(err).Fatal("Failed to open database")
	}
	defer db.Close()
	// Create DirectoryState
	dir := directory.NewDirectoryState("./test/*.log", serverUrl, logger)

	// Load state from database
	if err := dir.LoadState(db, ctx); err != nil {
		logger.WithError(err).Error("Failed to load state from database")
		os.Exit(1)
	}

	// Start watching the directory
	go func() {
		if err := dir.Watch(ctx); err != nil {
			logger.WithError(err).Error("Directory watching stopped unexpectedly")
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
					logger.WithError(err).Error("Failed to save state to database")
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

	dir.WaitForShutdown()

	if err := dir.SaveState(db); err != nil {
		logger.WithError(err).Error("Failed to save final state to database")
	}

	logger.Info("Application shutdown complete")
}
