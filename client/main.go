package main

import (
	"context"
	"log-forwarder-client/directory"
	"log-forwarder-client/reader"
	"os"
	"os/signal"
	"syscall"
)

var state *directory.DirectoryState

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	state = &directory.DirectoryState{Path: "./test/", RunningReaders: []*reader.Reader{}}

	directory.WatchDir(ctx, state)

	// Wait for a termination signal (SIGINT or SIGTERM)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	cancel()

	for _, reader := range state.RunningReaders {
		reader.Stop()
	}
}
