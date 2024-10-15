package input

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Utils Functions
func createLogger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelError}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

func writeTestContent(file *os.File, content []string) error {
	_, err := file.WriteString(stringJoinWithNewLine(content))
	return err
}

func stringJoinWithNewLine(content []string) string {
	return strings.Join(content, "\n") + "\n" // Join content with newlines
}

// Test functions

func GetState(t *testing.T) {
	logger := createLogger()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	tail, err := NewTail("../test/*.log", logger, &wg, ctx)
	assert.NoError(t, err, "Failed to start tail")

	go func() {
		for range tail.Read() {
		}
	}()

	time.Sleep(time.Second * 3)

	tail.Stop()
}

func TestTail(t *testing.T) {
	t.Run("TestTailGetState", GetState)
}
