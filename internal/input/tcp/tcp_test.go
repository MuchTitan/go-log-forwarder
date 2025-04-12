package inputtcp

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTCP_Init(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		wantErr bool
	}{
		{
			name: "default config",
			config: map[string]any{
				"Name":       "test",
				"Tag":        "test",
				"ListenAddr": "127.0.0.1",
				"Port":       6666,
			},
			wantErr: false,
		},
		{
			name: "invalid port type",
			config: map[string]any{
				"Name":       "test",
				"Tag":        "test",
				"ListenAddr": "127.0.0.1",
				"Port":       "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid buffer size type",
			config: map[string]any{
				"Name":       "test",
				"Tag":        "test",
				"ListenAddr": "127.0.0.1",
				"Port":       6666,
				"BufferSize": "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tcp := &TCP{}
			err := tcp.Init(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTCP_ConnectionHandling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create TCP input with test configuration
	tcp := &TCP{}
	err := tcp.Init(map[string]any{
		"Name":       "test",
		"Tag":        "test",
		"ListenAddr": "127.0.0.1",
		"Port":       0, // Let system choose port
	})
	require.NoError(t, err)

	// Channel to receive events
	events := make(chan internal.Event, 100)

	// Start TCP input
	err = tcp.Start(ctx, events)
	require.NoError(t, err)
	defer tcp.Exit()

	// Get the actual port the listener is using
	addr := tcp.listener.Addr().String()
	_, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	// Test connection and data sending
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%s", port))
	require.NoError(t, err)
	defer conn.Close()

	// Send test data
	testData := "test message\n"
	_, err = conn.Write([]byte(testData))
	require.NoError(t, err)

	// Wait for event
	select {
	case event := <-events:
		assert.Equal(t, testData, event.RawData)
		assert.Equal(t, "test", event.Metadata.Tag)
		assert.Equal(t, "test", event.Metadata.InputSource)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestTCP_ConnectionLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create TCP input with test configuration
	tcp := &TCP{}
	err := tcp.Init(map[string]any{
		"Name":       "test",
		"Tag":        "test",
		"ListenAddr": "127.0.0.1",
		"Port":       0, // Let system choose port
	})
	require.NoError(t, err)

	// Start TCP input
	err = tcp.Start(ctx, make(chan internal.Event))
	require.NoError(t, err)
	defer tcp.Exit()

	// Get the actual port
	addr := tcp.listener.Addr().String()
	_, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	// Create max connections
	var conns []net.Conn
	for i := 0; i < maxConnectionCountTCP; i++ {
		conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%s", port))
		require.NoError(t, err)
		conns = append(conns, conn)
	}

	// Try to create one more connection - should fail
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%s", port))
	require.NoError(t, err)
	defer conn.Close()

	// Wait a bit to ensure the connection is rejected
	time.Sleep(100 * time.Millisecond)

	// Clean up
	for _, conn := range conns {
		conn.Close()
	}
}

func TestTCP_Timeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create TCP input with very short timeout (1 second)
	tcp := &TCP{}
	err := tcp.Init(map[string]any{
		"Name":       "test",
		"Tag":        "test",
		"ListenAddr": "127.0.0.1",
		"Port":       0,
		"Timeout":    "0.0167", // 1 second timeout (0.0167 minutes)
	})
	require.NoError(t, err)

	// Channel to receive events
	events := make(chan internal.Event, 100)

	// Start TCP input
	err = tcp.Start(ctx, events)
	require.NoError(t, err)
	defer tcp.Exit()

	// Get the actual port
	addr := tcp.listener.Addr().String()
	_, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	// Create connection
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%s", port))
	require.NoError(t, err)
	defer conn.Close()

	// Set a deadline for the test itself
	testCtx, testCancel := context.WithTimeout(ctx, 5*time.Second)
	defer testCancel()

	// Wait for timeout or test deadline
	select {
	case <-testCtx.Done():
		t.Fatal("Test timed out waiting for connection to close")
	case <-time.After(2 * time.Second): // Wait slightly longer than the timeout
		// Try to read from the connection - should fail
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		_, err := conn.Read(make([]byte, 1))
		assert.Error(t, err, "Connection should be closed by timeout")
	}
}

func TestConnState(t *testing.T) {
	// Create a test connection
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// Test newConnState
	cs := newConnState(server)
	assert.NotNil(t, cs)
	assert.False(t, cs.IsClosed())

	// Test Close
	err := cs.Close()
	assert.NoError(t, err)
	assert.True(t, cs.IsClosed())

	// Test double close
	err = cs.Close()
	assert.NoError(t, err)
}
