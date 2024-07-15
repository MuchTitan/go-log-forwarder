package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"log-forwarder-client/config"
	"log-forwarder-client/directory"
	"log-forwarder-client/utils"

	"github.com/sirupsen/logrus"
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

	// Create DirectoryState
	dir := directory.NewDirectoryState("./test/", &directory.Config{
		DB:        cfg.DB,
		ServerURL: fmt.Sprintf("http://%s:%d/test", cfg.ServerUrl, cfg.ServerPort),
	}, logger)

	// Start watching the directory
	go func() {
		if err := dir.Watch(ctx); err != nil {
			logger.WithError(err).Error("Directory watching stopped unexpectedly")
		}
	}()

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Cancel the context to stop all operations
	cancel()

	// Wait for all operations to finish (you might want to implement a WaitGroup in DirectoryState)
	dir.WaitForShutdown()

	logger.Info("Application shutdown complete")
}
