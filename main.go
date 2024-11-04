package main

import (
	"log-forwarder-client/config"
	"log-forwarder-client/database"
	"log-forwarder-client/filter"
	"log-forwarder-client/input"
	"log-forwarder-client/output"
	"log-forwarder-client/parser"
	"log-forwarder-client/router"
	"log-forwarder-client/util"
	"os"
	"os/signal"
	"syscall"
)

var (
	cfg           *config.SystemConfig
	runningRouter []*router.Router
)

func StartRouters() {
	for _, in := range input.AvailableInputs {
		rt := router.NewRouter(cfg.Logger)
		rt.SetInput(in)
		for _, parser := range parser.AvailableParser {
			if util.TagMatch(in.GetTag(), parser.GetMatch()) {
				rt.AddParser(parser)
			}
		}
		for _, filter := range filter.AvailableFilters {
			if util.TagMatch(in.GetTag(), filter.GetMatch()) {
				rt.AddFilter(filter)
			}
		}
		for _, out := range output.AvailableOutputs {
			if util.TagMatch(in.GetTag(), out.GetMatch()) {
				rt.AddOutput(out)
			}
		}
		rt.Start()
		runningRouter = append(runningRouter, rt)
	}
}

func StopRouters() {
	for _, rt := range runningRouter {
		rt.Stop()
	}
}

func init() {
	// Get configuration
	cfg = config.LoadConfig("./cfg/cfg.yaml")
}

func main() {
	defer database.CloseDB()
	cfg.Logger.Info("Starting Log forwarder")

	// Start the Routers
	StartRouters()

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	cfg.Logger.Info("Log forwarder shutdown started")

	// Stops the running Routers
	StopRouters()

	err := database.CleanUpRetryData()
	if err != nil {
		cfg.Logger.Error("coudnt cleanup retry_data table", "error", err)
	}

	cfg.Logger.Info("Log forwarder shutdown complete")
}
