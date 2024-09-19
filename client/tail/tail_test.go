package tail

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func createLogger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelDebug} // Debug level logging
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

func writeTestContent(file *os.File, content []string) error {
	_, err := file.WriteString(stringJoinWithNewLine(content))
	return err
}

func stringJoinWithNewLine(content []string) string {
	return strings.Join(content, "\n") + "\n" // Join content with newlines
}

func TestTailOnFile(t *testing.T) {
	t.Run("TestTailOnFile", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test.log")
		assert.NoError(t, err, "Couldn't create temp file")
		defer os.Remove(tmpFile.Name())

		lineChan := make(chan Line, len([]string{"Test1", "Test2"})) // Buffered to avoid blocking

		logger := createLogger()
		tailer := NewFileTail(tmpFile.Name(), logger, lineChan, TailConfig{Offset: 0, ReOpen: false})

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		content := []string{"Test1", "Test2"}
		assert.NoError(t, writeTestContent(tmpFile, content), "Failed to write content")

		assert.NoError(t, tailer.Start(ctx), "Couldn't start tailer")

		for i := 0; i < len(content); i++ {
			select {
			case line := <-lineChan:
				assert.Equal(t, content[line.LineNum-1], line.LineData)
			case <-ctx.Done():
				t.Fatal("Test timed out before all lines were received")
			}
		}

		tailer.Stop()
	})
}
