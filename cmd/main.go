package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/MuchTitan/go-log-forwarder/internal/config"
	"github.com/sirupsen/logrus"
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

	logrus.Info("Starting log forwarder")

	if err := engine.Start(); err != nil {
		panic(err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logrus.Info("Stopping log forwarder")
	engine.Stop()
}
