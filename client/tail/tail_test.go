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

func TestTailOnFile(t *testing.T) {
	logger := createLogger()
	t.Run("TestTailOnFileWithOffset", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test.log")
		assert.NoError(t, err, "Couldnt create tmp file")

		content := []string{"Test1", "Test2", "Test3", "Test4"}
		lineChan := make(chan LineData)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		tailer, err := NewTailFile(tmpFile.Name(), logger, lineChan, 0, ctx)
		assert.NoError(t, err, "Error while opening new FileTail object")

		assert.NoError(t, writeTestContent(tmpFile, content), "Failed to write content")

		tailer.Start()

		for i := 0; i < len(content); i++ {
			select {
			case line := <-lineChan:
				assert.Equal(t, content[i], line.LineData)
			case <-ctx.Done():
				t.Fatal("Test timed out before all lines were received")
			}
		}

		tailer.Stop()
		os.Remove(tmpFile.Name())
	})

	t.Run("TestTailOnFileWithOffset", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test.log")
		assert.NoError(t, err, "Couldnt create tmp file")

		content := []string{"Test1", "Test2", "Test3", "Test4"}
		lineChan := make(chan LineData)

		offset := 2

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		tailer, err := NewTailFile(tmpFile.Name(), logger, lineChan, int64(offset), ctx)
		assert.NoError(t, err, "Error while opening new FileTail object")

		assert.NoError(t, writeTestContent(tmpFile, content), "Failed to write content")

		tailer.Start()

		for i := offset; i < len(content); i++ {
			select {
			case line := <-lineChan:
				assert.Equal(t, content[i], line.LineData)
			case <-ctx.Done():
				t.Fatal("Test timed out before all lines were received")
			}
		}

		tailer.Stop()
		os.Remove(tmpFile.Name())
	})
}
