package tail

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func createLogger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
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
	t.Run("TestTailOnFile", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test.log")
		assert.NoError(t, err, "Couldnt create temp file")
		defer os.Remove(tmpFile.Name())

		content := []string{"Test1", "Test2", "Test3", "Test4"}
		lineChan := make(chan Line) // Buffered to avoid blocking

		tailer := NewFileTail(tmpFile.Name(), logger, lineChan, TailConfig{
			LastSendLine: 0,
			ReOpenValue:  false,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		assert.NoError(t, writeTestContent(tmpFile, content), "Failed to write content")

		assert.NoError(t, tailer.Start(ctx), "Couldn't start tailer")

		for i := 0; i < len(content); i++ {
			select {
			case line := <-lineChan:
				assert.Equal(t, content[i], line.LineData)
			case <-ctx.Done():
				t.Fatal("Test timed out before all lines were received")
			}
		}

		tailer.Stop()
	})

	t.Run("TestTailOnFileWithOffset", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test.log")
		assert.NoError(t, err, "Couldnt create temp file")
		defer os.Remove(tmpFile.Name())

		content := []string{"Test1", "Test2", "Test3", "Test4"}
		lineChan := make(chan Line)

		offset := 2

		tailer := NewFileTail(tmpFile.Name(), logger, lineChan, TailConfig{
			LastSendLine: int64(offset),
			ReOpenValue:  false,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		assert.NoError(t, writeTestContent(tmpFile, content), "Failed to write content")

		assert.NoError(t, tailer.Start(ctx), "Couldn't start tailer")

		for i := offset; i < len(content); i++ {
			select {
			case line := <-lineChan:
				assert.Equal(t, content[i], line.LineData)
			case <-ctx.Done():
				t.Fatal("Test timed out before all lines were received")
			}
		}

		tailer.Stop()
	})

	t.Run("TestTailOnFileWithReOpen", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test.log")
		assert.NoError(t, err, "Couldnt create temp file")
		tmpFileName := tmpFile.Name()

		content := []string{"Test1", "Test2", "Test3", "Test4"}
		lineChan := make(chan Line)

		tailer := NewFileTail(tmpFileName, logger, lineChan, TailConfig{
			LastSendLine: 0,
			ReOpenValue:  true,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		assert.NoError(t, writeTestContent(tmpFile, content), "Failed to write content")

		assert.NoError(t, tailer.Start(ctx), "Couldn't start tailer")

		for i := 0; i < len(content); i++ {
			select {
			case line := <-lineChan:
				assert.Equal(t, content[i], line.LineData)
			case <-ctx.Done():
				t.Fatal("Test timed out before all lines were received")
			}
		}

		os.Remove(tmpFileName)
		time.Sleep(time.Second * 2)
		tmpFile, _ = os.Create(tmpFileName)
		defer os.Remove(tmpFileName)

		assert.NoError(t, writeTestContent(tmpFile, content), "Failed to write content2")

		for i := 0; i < len(content); i++ {
			select {
			case line := <-lineChan:
				assert.Equal(t, content[i], line.LineData)
			case <-ctx.Done():
				t.Fatal("Test timed out before all lines were received")
			}
		}

		tailer.Stop()
	})
}
