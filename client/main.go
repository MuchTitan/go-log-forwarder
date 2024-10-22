package main

import (
	"log-forwarder-client/config"
	"log-forwarder-client/database"
	"log-forwarder-client/input"
	"log-forwarder-client/output"
	"log-forwarder-client/parser"
	"log-forwarder-client/router"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	wg     *sync.WaitGroup
	cfg    *config.ApplicationConfig
	logger *slog.Logger
)

func main() {
	wg = &sync.WaitGroup{}

	// Get configuration
	cfg = config.GetApplicationConfig()

	// Setup Logger
	logger = cfg.Logger

	err := database.OpenDB(cfg.DBFile)
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.CloseDB()

	logger.Info("Starting Log forwarder")

	regex := parser.Regex{
		InputMatch: "foo",
		Pattern:    `^(?<host>[^ ]*) [^ ]* (?<user>[^ ]*) \[(?<time>[^\]]*)\] "(?<method>\S+)(?: +(?<path>[^\"]*?)(?: +\S*)?)?" (?<code>[^ ]*) (?<size>[^ ]*)(?: "(?<referer>[^\"]*)" "(?<agent>[^\"]*)")?$`,
	}

	json := parser.Json{
		InputMatch: "*",
	}

	parser.AvailableParser = append(parser.AvailableParser, regex)
	parser.AvailableParser = append(parser.AvailableParser, json)

	rt := router.NewRouter(wg, logger)
	in, err := input.NewTail("./test/*.log", config.GetLogger())
	rt.SetInput(in)

	// splunk := output.NewSplunk(
	// 	"localhost",
	// 	8088,
	// 	"397eb6a0-140f-4b0c-a0ff-dd8878672729",
	// 	false,
	// 	false,
	// 	"",
	// 	"",
	// 	"apache-log",
	// 	"test",
	// 	map[string]interface{}{},
	// 	logger,
	// )

	stdout := output.NewStdout()

	rt.AddOutput(stdout)

	rt.Start()

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("Log forwarder shutdown started")

	rt.Stop()

	err = database.CleanUpRetryData()
	if err != nil {
		logger.Error("coudnt cleanup retry_data table", "error", err)
	}

	logger.Info("Log forwarder shutdown complete")
}
