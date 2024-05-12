package main

import (
	"context"
	"fmt"
	"log"
	"log-forwarder-client/reader"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

)

func getDirContent(path string) ([]string, error) {
  result := []string{} 
  dirContent,err := os.ReadDir(path)
  if err != nil {
    return result,err
  }
  for _,t := range dirContent {
    fmt.Println(filepath.Join(path,t.Name()))
    if t.IsDir(){
      tmp,err := getDirContent(filepath.Join(path,t.Name()))
      if err != nil {
        return result,err
      }
      result = append(result, tmp...)
    }else{
      result = append(result, filepath.Join(path,t.Name()))
    }
  }
  return result, nil
}

func startReading(ctx context.Context,path string) {
	go func() {
		err := reader.ReadFile(ctx,path)
		if err != nil {
			log.Println("Error reading file:", err)
		}
	}()
}

func main() {
	// Create a context and cancel function to manage the lifecycle of operations
  ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure the context is canceled when main exits

  // startReading(ctx,"./test.log")
  dirContent, err := getDirContent("./test/")
  if err != nil {
    // Handle error
  }

  for _, dirData := range dirContent {
    startReading(ctx, dirData)
  }

	// Start the ReadFile operation in a separate goroutine

	// Wait for a termination signal (SIGINT or SIGTERM)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Signal received, cancel the context to stop the ReadFile operation
	cancel()

	fmt.Println("Exiting...")
	time.Sleep(1 * time.Second) // Give some time for cleanup if needed
}

