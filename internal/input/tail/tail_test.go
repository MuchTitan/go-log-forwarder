package tail

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTailRepository implements TailRepository for testing
type MockTailRepository struct {
	mock.Mock
}

func (m *MockTailRepository) CreateTables() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTailRepository) Close() error {
	return nil
}

func (m *MockTailRepository) GetFileState(path string, inode uint64) (*fileState, error) {
	args := m.Called(path, inode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fileState), args.Error(1)
}

func (m *MockTailRepository) DeleteFileState(path string, inode uint64) error {
	args := m.Called(path, inode)
	return args.Error(0)
}

func (m *MockTailRepository) BatchUpsertFileStates(states []fileState) error {
	args := m.Called(states)
	return args.Error(0)
}

func (m *MockTailRepository) CleanupOldEntries(threshold int) (int64, error) {
	args := m.Called(threshold)
	return args.Get(0).(int64), args.Error(1)
}

// Test helpers
func createTempFile(t *testing.T, dir, content string) (string, func()) {
	tmpFile, err := os.CreateTemp(dir, "test-*.log")
	if err != nil {
		t.Fatal(err)
	}

	if content != "" {
		if _, err := tmpFile.WriteString(content); err != nil {
			t.Fatal(err)
		}
	}

	cleanup := func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}

	return tmpFile.Name(), cleanup
}

func createTempDir(t *testing.T, pattern string) (string, func()) {
	tmpDir, err := os.MkdirTemp("", pattern)
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestTail_Init(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		wantErr bool
	}{
		{
			name: "valid config",
			config: map[string]any{
				"Glob": "*.log",
				"Name": "test-tail",
				"Tag":  "test",
			},
			wantErr: false,
		},
		{
			name: "missing glob",
			config: map[string]any{
				"Name": "test-tail",
			},
			wantErr: true,
		},
		{
			name: "with DB enabled",
			config: map[string]any{
				"Glob":     "*.log",
				"EnableDB": true,
				"DBFile":   "test.db",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tail := &Tail{}
			err := tail.Init(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTail_ReadFile(t *testing.T) {
	content := "line1\nline2\nline3\n"
	tmpFile, cleanup := createTempFile(t, "", content)
	defer cleanup()

	mockRepo := new(MockTailRepository)
	mockRepo.On("GetFileState", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("not found"))

	tail := &Tail{
		repository: mockRepo,
		state:      make(map[string]*fileState),
		ctx:        context.Background(),
	}

	output := make(chan internal.Event, 10)
	var wg sync.WaitGroup
	wg.Add(1)
	tail.wg.Add(1)

	go func() {
		defer wg.Done()
		err := tail.readFile(tmpFile, output)
		assert.NoError(t, err)
	}()

	receivedLines := 0
	timeout := time.After(5 * time.Second)

	for receivedLines < 3 {
		select {
		case <-output:
			receivedLines++
		case <-timeout:
			t.Fatal("timeout waiting for lines")
		}
	}

	wg.Wait()
	assert.Equal(t, 3, receivedLines)
}

func TestTail_FileStatLoop(t *testing.T) {
	tmpDir, cleanup := createTempDir(t, "test-tail-filestate")
	defer cleanup()

	// Create test files
	_, cleanup1 := createTempFile(t, tmpDir, "test1")
	defer cleanup1()
	_, cleanup2 := createTempFile(t, tmpDir, "test2")
	defer cleanup2()

	tail := &Tail{
		glob:        filepath.Join(tmpDir, "*.log"),
		fileEventCh: make(chan string, 10),
		fileStats:   make(map[string]fileInfo),
		ctx:         context.Background(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	tail.wg.Add(1)
	go func() {
		defer wg.Done()
		tail.fileStatLoop(ctx)
	}()

	// Verify file events are received
	timeout := time.After(5 * time.Second)
	eventsReceived := 0

	for eventsReceived < 2 {
		select {
		case <-tail.fileEventCh:
			eventsReceived++
		case <-timeout:
			t.Fatal("timeout waiting for file events")
		}
	}

	cancel()
	wg.Wait()
}

func TestTail_PersistStates(t *testing.T) {
	mockRepo := new(MockTailRepository)
	mockRepo.On("BatchUpsertFileStates", mock.Anything).Return(nil)

	tail := &Tail{
		repository:         mockRepo,
		fileStateCh:        make(chan fileState, 10),
		stateSavingEnabled: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	tail.wg.Add(1)
	go func() {
		defer wg.Done()
		tail.ctx = ctx
		tail.persistStates()
	}()

	// Send test states
	testStates := []fileState{
		{Path: "test1.log", Offset: 100, LastReadLine: 10},
		{Path: "test2.log", Offset: 200, LastReadLine: 20},
	}

	for _, state := range testStates {
		tail.fileStateCh <- state
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)
	cancel()
	wg.Wait()

	mockRepo.AssertExpectations(t)
}

func TestTail_Integration(t *testing.T) {
	tmpDir, cleanup := createTempDir(t, "test-tail-integration")
	defer cleanup()

	// Create test files
	testFile := filepath.Join(tmpDir, "test.log")
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	config := map[string]any{
		"Glob":     filepath.Join(tmpDir, "*.log"),
		"Name":     "test-tail",
		"Tag":      "test",
		"EnableDB": true,
		"DBFile":   filepath.Join(tmpDir, "test.db"),
	}

	tail := &Tail{}
	err = tail.Init(config)
	assert.NoError(t, err)

	output := make(chan internal.Event)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = tail.Start(ctx, output)
	assert.NoError(t, err)

	// Write some data
	testData := []string{
		"line1\n",
		"line2\n",
		"line3\n",
	}

	for _, line := range testData {
		_, err := f.WriteString(line)
		assert.NoError(t, err)
	}
	f.Sync()

	// Verify events
	receivedEvents := 0
	timeout := time.After(5 * time.Second)

	for receivedEvents < len(testData) {
		select {
		case evt := <-output:
			assert.NotEmpty(t, evt.RawData)
			receivedEvents++
		case <-timeout:
			t.Fatal("timeout waiting for events")
		}
	}

	err = tail.Exit()
	assert.NoError(t, err)
}
