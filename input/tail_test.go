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
	require.NoError(t, err, "Failed to create temporary directory")

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
	require.NoError(t, err, "Failed to create test file")
	defer f.Close()

	for _, line := range content {
		_, err = f.WriteString(line + "\n")
		require.NoError(t, err, "Failed to write content to test file")
	}
	return path
}

func TestTailE2E(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	logger := slog.Default()

	t.Run("Basic file reading and watching", func(t *testing.T) {
		testFile := createTestFile(t, tmpDir, "test.log", []string{"line1", "line2"})

		config := map[string]interface{}{
			"Glob": filepath.Join(tmpDir, "*.log"),
			"Tag":  "test-tag",
		}

		tail, err := input.ParseTail(config, logger)
		require.NoError(t, err, "Failed to parse tail configuration")
		defer tail.Stop()

		tail.Start()

		eventCh := tail.Read()
		var events []util.Event
		timeout := time.After(2 * time.Second)

		for i := 0; i < 2; i++ {
			select {
			case event := <-eventCh:
				events = append(events, event)
			case <-timeout:
				t.Fatal("Timeout waiting for initial events")
			}
		}

		assert.Len(t, events, 2, "Expected 2 initial events")
		assert.Equal(t, "line1", string(events[0].RawData))
		assert.Equal(t, "line2", string(events[1].RawData))

		// Append new content and verify new events
		appendContent(t, testFile, "line3")

		select {
		case event := <-eventCh:
			assert.Equal(t, "line3", string(event.RawData))
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for new event after append")
		}
	})
	t.Run("TestTailWithFileCreationDeletion", func(t *testing.T) {
		tmpDir, cleanup := setupTestEnvironment(t)
		defer cleanup()

		logger := slog.Default()
		config := map[string]interface{}{
			"Glob": filepath.Join(tmpDir, "dynamic*.log"),
			"Tag":  "dynamic-tag",
		}

		// Initialize tail and start watching
		tail, err := input.ParseTail(config, logger)
		require.NoError(t, err)

		tail.Start()
		defer tail.Stop()

		eventCh := tail.Read()

		// Write events to multiple files
		initialContent := map[string][]string{
			"dynamic1.log": {"event1-file1", "event2-file1"},
			"dynamic2.log": {"event1-file2", "event2-file2"},
			"dynamic3.log": {"event1-file3", "event2-file3"},
		}

		// Create initial files
		for name, content := range initialContent {
			createTestFile(t, tmpDir, name, content)
		}

		// Collect initial events from the files
		expectedEvents := make(map[string]bool)
		for _, content := range initialContent {
			for _, line := range content {
				expectedEvents[line] = true
			}
		}

		receivedEvents := make(map[string]bool)
		timeout := time.After(3 * time.Second)
		for i := 0; i < len(expectedEvents); i++ {
			select {
			case event := <-eventCh:
				receivedEvents[string(event.RawData)] = true
			case <-timeout:
				t.Fatal("Timeout waiting for initial events")
			}
		}

		assert.Equal(t, expectedEvents, receivedEvents)

		// Randomly delete and re-create files with new content
		newContent := map[string][]string{
			"dynamic1.log": {"new-event1-file1", "new-event2-file1"},
			"dynamic2.log": {"new-event1-file2", "new-event2-file2"},
		}

		for name := range initialContent {
			if _, exists := newContent[name]; exists {
				os.Remove(filepath.Join(tmpDir, name))
				time.Sleep(1 * time.Second) // Allow for filesystem events to propagate

				createTestFile(t, tmpDir, name, newContent[name])
			}
		}

		// Collect new events from re-created files
		expectedNewEvents := make(map[string]bool)
		for _, content := range newContent {
			for _, line := range content {
				expectedNewEvents[line] = true
			}
		}

		receivedNewEvents := make(map[string]bool)
		timeout = time.After(3 * time.Second)
		for i := 0; i < len(expectedNewEvents); i++ {
			select {
			case event := <-eventCh:
				receivedNewEvents[string(event.RawData)] = true
			case <-timeout:
				t.Fatal("Timeout waiting for new events")
			}
		}

		time.Sleep(time.Second * 3)

		assert.Equal(t, expectedNewEvents, receivedNewEvents)
	})

	t.Run("Multiple file handling", func(t *testing.T) {
		createTestFile(t, tmpDir, "multi1.log", []string{"multi1-line1"})
		createTestFile(t, tmpDir, "multi2.log", []string{"multi2-line1"})

		config := map[string]interface{}{
			"Glob": filepath.Join(tmpDir, "multi*.log"),
			"Tag":  "multi-tag",
		}

		tail, err := input.ParseTail(config, logger)
		require.NoError(t, err, "Failed to parse tail configuration")
		defer tail.Stop()

		tail.Start()
		events := collectEvents(t, tail.Read(), 2, 2*time.Second)
		assert.True(t, events["multi1-line1"])
		assert.True(t, events["multi2-line1"])

		createTestFile(t, tmpDir, "multi3.log", []string{"multi3-line1"})
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
		assert.Error(t, err, "Expected error for invalid glob pattern")
	})

	t.Run("Missing glob", func(t *testing.T) {
		config := map[string]interface{}{
			"Tag": "error-tag",
		}

		_, err := input.ParseTail(config, logger)
		assert.Error(t, err, "Expected error for missing glob pattern")
	})

	t.Run("Unreadable file", func(t *testing.T) {
		testFile := createTestFile(t, tmpDir, "unreadable.log", []string{"test"})
		err := os.Chmod(testFile, 0000)
		require.NoError(t, err, "Failed to set file permissions")

		config := map[string]interface{}{
			"Glob": filepath.Join(tmpDir, "*.log"),
			"Tag":  "error-tag",
		}

		tail, err := input.ParseTail(config, logger)
		require.NoError(t, err, "Failed to parse tail configuration")
		defer tail.Stop()

		tail.Start()
		time.Sleep(1 * time.Second) // Allow time for the tail to detect files
		// The unreadable file should be skipped without error
	})
}

// Helper function to append content to an existing file
func appendContent(t *testing.T, path, content string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err, "Failed to open file for appending")
	defer f.Close()

	_, err = f.WriteString(content + "\n")
	require.NoError(t, err, "Failed to write appended content")
}

// Helper function to collect a set number of events within a given timeout
func collectEvents(t *testing.T, eventCh <-chan util.Event, count int, timeout time.Duration) map[string]bool {
	events := make(map[string]bool)
	timer := time.After(timeout)

	for i := 0; i < count; i++ {
		select {
		case event := <-eventCh:
			events[string(event.RawData)] = true
		case <-timer:
			t.Fatalf("Timeout waiting for events, expected %d, got %d", count, i)
		}
	}
	return events
}
