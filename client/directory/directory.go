// directory.go
package directory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log-forwarder-client/reader"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type DirectoryState struct {
	Path           string
	Time           time.Time
	RunningReaders map[string]*reader.Reader
	DBId           int
	Config         *Config
	Logger         *logrus.Logger
	mu             sync.Mutex
}

type Config struct {
	DB        *sql.DB
	ServerURL string
}

func NewDirectoryState(path string, config *Config, logger *logrus.Logger) *DirectoryState {
	return &DirectoryState{
		Path:           path,
		RunningReaders: make(map[string]*reader.Reader),
		Config:         config,
		Logger:         logger,
	}
}

func getDirContent(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, fmt.Errorf("permission denied accessing a directory: %w", err)
		}
		return nil, fmt.Errorf("error walking the path %s: %w", root, err)
	}

	return files, nil
}

func (d *DirectoryState) Watch(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := d.checkDirectory(); err != nil {
				d.Logger.WithError(err).Error("Failed to check directory")
			}
		}
	}
}

func (d *DirectoryState) checkDirectory() error {
	files, err := getDirContent(d.Path)
	if err != nil {
		return fmt.Errorf("failed to get directory content: %w", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, file := range files {
		if _, exists := d.RunningReaders[file]; !exists {
			if err := d.startReader(file); err != nil {
				d.Logger.WithError(err).WithField("file", file).Error("Failed to start reader")
			}
		}
	}

	for file, r := range d.RunningReaders {
		if !slices.Contains(files, file) {
			r.Stop()
			delete(d.RunningReaders, file)
		}
	}

	return nil
}

func (d *DirectoryState) startReader(file string) error {
	r := reader.New(file, &reader.Config{
		ServerURL: d.Config.ServerURL,
		DB:        d.Config.DB,
	}, d.Logger)

	if err := r.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start reader: %w", err)
	}

	d.RunningReaders[file] = r
	return nil
}

func (d *DirectoryState) WaitForShutdown() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, reader := range d.RunningReaders {
		reader.Stop()
	}
}
