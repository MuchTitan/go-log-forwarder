package directory

import (
	"context"
	"log"
	"log-forwarder-client/reader"
	"os"
	"path/filepath"
	"slices"
	"time"
)

type DirectoryState struct {
	Path           string
	RunningReaders []*reader.Reader
}

func getDirContent(path string) ([]string, error) {
	result := []string{}
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
			result = append(result, filepath.Join(path, t.Name()))
		}
	}
	return result, nil
}

func CheckIfFileIsRead(runningReaders []*reader.Reader, path string) bool {
	for _, reader := range runningReaders {
		if path == reader.GetPath() && reader.IsRunning() {
			return true
		}
	}
	return false
}

func CleanUpRunningReaders(shouldBeRunning []string, runningReaders []*reader.Reader) []*reader.Reader {
	result := []*reader.Reader{}
	for _, reader := range runningReaders {
		if !slices.Contains(shouldBeRunning, reader.GetPath()) {
			reader.Stop()
		} else {
			result = append(result, reader)
		}
	}
	return result
}

func StartReading(path string) *reader.Reader {
	reader := reader.New(reader.Config{Path: path})
	reader.Start()
	return reader
}

func WatchDir(ctx context.Context, state *DirectoryState) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				dirContent, err := getDirContent(state.Path)
				if err != nil {
					log.Fatalln(err)
				}
				shouldBeRunning := []string{}
				for _, path := range dirContent {
					shouldBeRunning = append(shouldBeRunning, path)
					if !CheckIfFileIsRead(state.RunningReaders, path) {
						state.RunningReaders = append(state.RunningReaders, StartReading(path))
					}
				}
				state.RunningReaders = CleanUpRunningReaders(shouldBeRunning, state.RunningReaders)
				time.Sleep(time.Second * 5)
			}
		}
	}()
}
