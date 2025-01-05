package main

import (
	"log-forwarder/config"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	engine, err := config.NewPluginEngine("./cfg/cfg.yaml")
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
}
