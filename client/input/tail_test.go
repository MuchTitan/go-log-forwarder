package input_test

import (
	"log-forwarder-client/database"
	"log-forwarder-client/input"
	"log-forwarder-client/util"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEnvironment(t *testing.T) (string, func()) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "tail-test-*")
	require.NoError(t, err)

	// Setup test database
	db := database.SetupTestDB(t)

	// Return cleanup function
	cleanup := func() {
		os.RemoveAll(tmpDir)
		db.Close()
	}

	return tmpDir, cleanup
}

func createTestFile(t *testing.T, dir, name string, content []string) string {
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	for _, line := range content {
		_, err = f.WriteString(line + "\n")
		require.NoError(t, err)
	}
	return path
}

func TestTailE2E(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	logger := slog.Default()

	t.Run("Basic file reading and watching", func(t *testing.T) {
		// Create initial test file
		testFile := createTestFile(t, tmpDir, "test.log", []string{
			"line1",
			"line2",
		})

		// Create tail configuration
		config := map[string]interface{}{
			"Glob": filepath.Join(tmpDir, "*.log"),
			"Tag":  "test-tag",
		}

		// Initialize tail
		tail, err := input.ParseTail(config, logger)
		require.NoError(t, err)

		// Start tailing
		tail.Start()
		defer tail.Stop()

		// Channel to receive events
		eventCh := tail.Read()

		// Wait for initial lines
		var events []util.Event
		timeout := time.After(2 * time.Second)

		// Collect initial events
		for i := 0; i < 2; i++ {
			select {
			case event := <-eventCh:
				events = append(events, event)
			case <-timeout:
				t.Fatal("Timeout waiting for events")
			}
		}

		// Verify initial events
		assert.Equal(t, 2, len(events))
		assert.Equal(t, "line1", string(events[0].RawData))
		assert.Equal(t, "line2", string(events[1].RawData))

		// Append new content
		f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
		require.NoError(t, err)
		_, err = f.WriteString("line3\n")
		require.NoError(t, err)
		f.Close()

		// Wait for new event
		select {
		case event := <-eventCh:
			assert.Equal(t, "line3", string(event.RawData))
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for new event")
		}
	})

	t.Run("Multiple file handling", func(t *testing.T) {
		// Create multiple test files
		createTestFile(t, tmpDir, "multi1.log", []string{"multi1-line1"})
		createTestFile(t, tmpDir, "multi2.log", []string{"multi2-line1"})

		config := map[string]interface{}{
			"Glob": filepath.Join(tmpDir, "multi*.log"),
			"Tag":  "multi-tag",
		}

		tail, err := input.ParseTail(config, logger)
		require.NoError(t, err)

		tail.Start()
		defer tail.Stop()

		// Collect events from both files
		events := make(map[string]bool)
		timeout := time.After(2 * time.Second)

		for i := 0; i < 2; i++ {
			select {
			case event := <-tail.Read():
				events[string(event.RawData)] = true
			case <-timeout:
				t.Fatal("Timeout waiting for events")
			}
		}

		// Verify we got events from both files
		assert.True(t, events["multi1-line1"])
		assert.True(t, events["multi2-line1"])

		// Test adding a new file
		createTestFile(t, tmpDir, "multi3.log", []string{"multi3-line1"})

		// Wait for detection of new file
		time.Sleep(6 * time.Second) // Allow for directory check interval

		select {
		case event := <-tail.Read():
			assert.Equal(t, "multi3-line1", string(event.RawData))
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for new file event")
		}
	})
}

func TestTailErrors(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	logger := slog.Default()

	t.Run("Invalid glob pattern", func(t *testing.T) {
		config := map[string]interface{}{
			"Glob": filepath.Join(tmpDir, "[[invalid-glob"),
			"Tag":  "error-tag",
		}

		_, err := input.ParseTail(config, logger)
		assert.Error(t, err)
	})

	t.Run("Missing glob", func(t *testing.T) {
		config := map[string]interface{}{
			"Tag": "error-tag",
		}

		_, err := input.ParseTail(config, logger)
		assert.Error(t, err)
	})

	t.Run("Unreadable file", func(t *testing.T) {
		testFile := createTestFile(t, tmpDir, "unreadable.log", []string{"test"})
		err := os.Chmod(testFile, 0000)
		require.NoError(t, err)

		config := map[string]interface{}{
			"Glob": filepath.Join(tmpDir, "*.log"),
			"Tag":  "error-tag",
		}

		tail, err := input.ParseTail(config, logger)
		require.NoError(t, err)

		tail.Start()
		defer tail.Stop()

		// The unreadable file should be skipped without error
		time.Sleep(1 * time.Second)
	})
}
