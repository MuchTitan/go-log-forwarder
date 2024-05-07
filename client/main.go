package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
  "log-forwarder-client/reader"	
)

func main() {
	// Create a context and cancel function to manage the lifecycle of operations
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure the context is canceled when main exits


	// Start the ReadFile operation in a separate goroutine
	go func() {
		err := reader.ReadFile(ctx, "./test.log")
		if err != nil {
			log.Println("Error reading file:", err)
		}
	}()

	// Wait for a termination signal (SIGINT or SIGTERM)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Signal received, cancel the context to stop the ReadFile operation
	cancel()

	fmt.Println("Exiting...")
	time.Sleep(1 * time.Second) // Give some time for cleanup if needed
}

