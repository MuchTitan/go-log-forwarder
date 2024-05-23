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

func CreateFile(parent context.Context,path string) *models.FileState{
  ctx, cancel := context.WithCancel(parent)
  return &models.FileState{
    Path: path,
    LastSendLine: 0,
    State: ctx,
    Cancel: cancel,
  }
}

func checkIfFileStateExists(slice []*models.FileState,path string) bool {
  for _, file := range(slice) {
    if file.Path == path {
      return true
    }
  }
  return false
}

func getDirContent(ctx context.Context,path string) ([]*models.FileState, error) {
  result := []*models.FileState{} 
  dirContent,err := os.ReadDir(path)
  if err != nil {
    return result,err
  }
  for _,t := range dirContent {
    if t.IsDir(){
      tmp,err := getDirContent(ctx,filepath.Join(path,t.Name()))
      if err != nil {
        return result,err
      }
      result = append(result, tmp...)
    }else{
      if !checkIfFileStateExists(result, filepath.Join(path,t.Name())) {
        t := CreateFile(ctx,filepath.Join(path,t.Name()))
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

func startReading(ctx context.Context,path string) {
	go func() {
		err := reader.ReadFile(ctx,path)
		if err != nil {
			log.Println("Error reading file:", err)
		}
	}()
}

func StopZombieContext(input1 []string, input2 map[string]context.CancelFunc) map[string]context.CancelFunc{
  result := make(map[string]context.CancelFunc)
  for key,cancel := range input2 {
    if !slices.Contains(input1,key) {
      cancel()
    }else{
      result[key]=cancel
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

  errChan := make(chan error)
  go func(){
    for {
      dirContent, err := getDirContent(ctx,"./test/")
      if err != nil {
        log.Fatalln(err)
      }
      // for _ ,x := range(dirContent) {
      //   log.Println(x.Path)
      // }
      stillRunningLogFiles := []string{}

      for _, data := range dirContent {
        log.Println(runningLogFile,data.Path)
        if MapContainsKey(runningLogFile,data.Path){
          if data.State.Err() != nil {
            errChan <- reader.ReadFile(data.State, data.Path)
            stillRunningLogFiles = append(stillRunningLogFiles, data.Path)
          }
        } else {
          log.Printf("Start Reading of File %s", data.Path)
          errChan <- reader.ReadFile(data.State, data.Path)
          stillRunningLogFiles = append(stillRunningLogFiles, data.Path)
          runningLogFile[data.Path] = data.Cancel
        }
      }
      runningLogFile = StopZombieContext(stillRunningLogFiles,runningLogFile)
      time.Sleep(time.Second * 2)
    }
  }()
    
  for err := range(errChan) {
    log.Println(err)
  }

	// Wait for a termination signal (SIGINT or SIGTERM)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Signal received, cancel the context to stop the ReadFile operation
	cancel()

	fmt.Println("Exiting...")
	time.Sleep(1 * time.Second) // Give some time for cleanup if needed
}

