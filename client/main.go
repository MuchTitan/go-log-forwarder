package main

import (
	"context"
	"fmt"
	"log"
	"log-forwarder-client/models"
	"log-forwarder-client/reader"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"
)

func checkIfFileStateExists(slice []*models.FileState, path string) bool {
	for _, file := range slice {
		if file.Path == path {
			return true
		}
	}
	return false
}

func getDirContent(path string) ([]*models.FileState, error) {
	result := []*models.FileState{}
	dirContent, err := os.ReadDir(path)
	if err != nil {
		return result, err
	}
	for _, t := range dirContent {
		if t.IsDir() {
			tmp, err := getDirContent(filepath.Join(path, t.Name()))
			if err != nil {
				return result, err
			}
			result = append(result, tmp...)
		} else {
			if !checkIfFileStateExists(result, filepath.Join(path, t.Name())) {
				t := models.CreateFile(filepath.Join(path, t.Name()))
				result = append(result, t)
			}
		}
	}
	return result, nil
}

func difference[T comparable](slice1, slice2 []T) []T {
	result := []T{}
	for _, item1 := range slice1 {
		if !slices.Contains(slice2, item1) {
			result = append(result, item1)
		}
	}
	return result

}

// func startReading(file *models.FileState)  {
// 	go func(){
// 		logLineChan, err := reader.ReadFile(file)
// 		if err != nil {
// 			return 
// 		}
// 		for {
// 			select{
// 			case <- file.State.Done():
// 				return 
// 			case logLine, ok:= <- logLineChan:
// 				if !ok {
// 					return 
// 				}
// 				log.Println(logLine.Data)
// 			}
// 		}
// 	}()
// }

func startReading(file *models.FileState) {
	go func() {
		err := reader.ReadFile(file)
		if err != nil {
			log.Fatalf("An error occurred during File Reading %s",err)
		}
	}()
}

func StopContext(input1 []string, input2 map[string]context.CancelFunc) map[string]context.CancelFunc {
	result := make(map[string]context.CancelFunc)
	for key, cancel := range input2 {
		if !slices.Contains(input1, key) {
			log.Printf("Stop Reading for File %s", key)
			cancel()
		} else {
			result[key] = cancel
		}
	}
	return result
}

func MapContainsKey[T comparable, E any](input map[T]E, searchTerm T) bool {
	_, exits := input[searchTerm]
	return exits
}

func main() {
	// Create a context and cancel function to manage the lifecycle of operations
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure the context is canceled when main exits
	runningLogFile := make(map[string]context.CancelFunc)

	go func() {
		for {
			dirContent, err := getDirContent("./test/")
			if err != nil {
				log.Fatalln(err)
			}
			stillRunningLogFiles := []string{}

			for _, data := range dirContent {
				if MapContainsKey(runningLogFile, data.Path) {
					stillRunningLogFiles = append(stillRunningLogFiles, data.Path)
					if data.State != nil && data.State.Err() != nil {
						data.SetContext(ctx)
						startReading(data)
					}
				} else {
					log.Printf("Start Reading of File %s", data.Path)
					data.SetContext(ctx)
					startReading(data)
					stillRunningLogFiles = append(stillRunningLogFiles, data.Path)
					runningLogFile[data.Path] = data.Cancel
				}
			}
			runningLogFile = StopContext(stillRunningLogFiles, runningLogFile)
			time.Sleep(time.Second * 10)
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
