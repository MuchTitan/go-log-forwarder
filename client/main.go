package main

import (
	"context"
	"io"
	"log-forwarder-client/config"
	"log-forwarder-client/input"
	"log-forwarder-client/output"
	"log-forwarder-client/parser"
	"log-forwarder-client/router"
	"log-forwarder-client/utils"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"go.etcd.io/bbolt"
)

type LogOut interface {
	io.Writer
}

var (
	// runningDirectorys []*directory.DirectoryState
	wg        *sync.WaitGroup
	parentCtx context.Context
	cfg       *config.ApplicationConfig
	logger    *slog.Logger
	db        *bbolt.DB
)

// func openDB() {
// 	var err error
// 	db, err = bbolt.Open(cfg.DBFile, 0600, nil)
// 	if err != nil {
// 		logger.Error("Failed to open database", "error", err)
// 		os.Exit(1)
// 	}
// }
//
// func saveToDB() {
// 	for _, dir := range runningDirectorys {
// 		if err := dir.SaveState(db); err != nil {
// 			logger.Error("Failed to save state to database", "error", err)
// 		}
// 	}
// }

func main() {
	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg = &sync.WaitGroup{}

	// Get configuration
	cfg = config.GetApplicationConfig()
	logger = cfg.Logger

	logger.Info("Starting Log forwarder")

	rt := router.NewRouter(wg, parentCtx)
	in := input.NewTail("./test/*.log", wg, parentCtx)
	rt.AddInput(in)

	jsonParser := parser.Json{}

	rt.AddParser(jsonParser)

	out := output.Splunk{
		Host:        "localhost",
		Port:        8088,
		SplunkToken: "397eb6a0-140f-4b0c-a0ff-dd8878672729",
		VerifyTLS:   false,
		EventHost:   utils.GetHostname(),
		EventIndex:  "test",
	}

	rt.AddOutput(out)

	rt.Start()

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("Log forwarder shutdown started")

	cancel()

	rt.Stop()

	logger.Info("Log forwarder shutdown complete")
}
