package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/MuchTitan/go-log-forwarder/internal/config"
)

type FlagOptions struct {
	configPath *string
}

var opts = FlagOptions{}

func init() {
	opts.configPath = flag.String("cfg", "/app/cfg.yaml", "provided the path to your config file")
	flag.Parse()
}

func main() {
	engine, err := config.NewPluginEngine(*opts.configPath)
	if err != nil {
		panic(err)
	}

	slog.Info("[Engine] Starting log forwarder")

	if err := engine.Start(); err != nil {
		panic(err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	slog.Info("[Engine] Stopping log forwarder")
	engine.Stop()
	if err := engine.DbManager.Close(); err != nil {
		slog.Error("could not close the database", "error", err)
	}
}
