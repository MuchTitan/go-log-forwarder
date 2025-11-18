package main

import (
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/MuchTitan/go-log-forwarder/internal/config"
	"github.com/sirupsen/logrus"
)

// StringSlice is a custom flag type that allows multiple values
type StringSlice []string

func (s *StringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *StringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type FlagOptions struct {
	configPath *string
	inputs     StringSlice
	parsers    StringSlice
	filters    StringSlice
	outputs    StringSlice
}

var opts = FlagOptions{}

func init() {
	opts.configPath = flag.String("cfg", "", "path to config file (optional if using CLI flags)")
	flag.Var(&opts.inputs, "input", "input plugin configuration (format: Type=value,Key=value), repeatable")
	flag.Var(&opts.parsers, "parser", "parser plugin configuration (format: Type=value,Key=value), repeatable")
	flag.Var(&opts.filters, "filter", "filter plugin configuration (format: Type=value,Key=value), repeatable")
	flag.Var(&opts.outputs, "output", "output plugin configuration (format: Type=value,Key=value), repeatable")
	flag.Parse()
}

func main() {
	// Parse CLI configurations
	cliConfig := config.CLIConfig{}

	// Parse input plugins
	for _, input := range opts.inputs {
		pluginConfig, err := config.ParsePluginConfig(input)
		if err != nil {
			logrus.Fatalf("Failed to parse input configuration '%s': %v", input, err)
		}
		if err := config.ValidatePluginConfig("input", pluginConfig); err != nil {
			logrus.Fatalf("Invalid input configuration: %v", err)
		}
		cliConfig.Inputs = append(cliConfig.Inputs, pluginConfig)
	}

	// Parse parser plugins
	for _, parser := range opts.parsers {
		pluginConfig, err := config.ParsePluginConfig(parser)
		if err != nil {
			logrus.Fatalf("Failed to parse parser configuration '%s': %v", parser, err)
		}
		if err := config.ValidatePluginConfig("parser", pluginConfig); err != nil {
			logrus.Fatalf("Invalid parser configuration: %v", err)
		}
		cliConfig.Parsers = append(cliConfig.Parsers, pluginConfig)
	}

	// Parse filter plugins
	for _, filter := range opts.filters {
		pluginConfig, err := config.ParsePluginConfig(filter)
		if err != nil {
			logrus.Fatalf("Failed to parse filter configuration '%s': %v", filter, err)
		}
		if err := config.ValidatePluginConfig("filter", pluginConfig); err != nil {
			logrus.Fatalf("Invalid filter configuration: %v", err)
		}
		cliConfig.Filters = append(cliConfig.Filters, pluginConfig)
	}

	// Parse output plugins
	for _, output := range opts.outputs {
		pluginConfig, err := config.ParsePluginConfig(output)
		if err != nil {
			logrus.Fatalf("Failed to parse output configuration '%s': %v", output, err)
		}
		if err := config.ValidatePluginConfig("output", pluginConfig); err != nil {
			logrus.Fatalf("Invalid output configuration: %v", err)
		}
		cliConfig.Outputs = append(cliConfig.Outputs, pluginConfig)
	}

	// Validate that at least some configuration is provided
	if *opts.configPath == "" && len(cliConfig.Inputs) == 0 && len(cliConfig.Parsers) == 0 &&
		len(cliConfig.Filters) == 0 && len(cliConfig.Outputs) == 0 {
		logrus.Fatal("No configuration provided. Use --cfg to specify config file or use CLI flags (--input, --parser, --filter, --output)")
	}

	// Create engine with combined config
	engine, err := config.NewPluginEngineWithCLI(*opts.configPath, cliConfig)
	if err != nil {
		logrus.Fatalf("Failed to create engine: %v", err)
	}

	logrus.Info("Starting log forwarder")

	if err := engine.Start(); err != nil {
		logrus.Fatalf("Failed to start engine: %v", err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logrus.Info("Stopping log forwarder")
	engine.Stop()
}
