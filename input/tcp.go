package input

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"log-forwarder-client/util"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"
)

const (
	defaultTCPBufferSize  int64 = 64 << 10 // 64KB
	defaultTCPTimeout           = 10       // 10 minute timeout
	maxConnectionCountTCP       = 50       // Maximum number of concurrent connections
)

type InTCP struct {
	ctx               context.Context
	listener          net.Listener
	sendCh            chan util.Event
	cancel            context.CancelFunc
	wg                *sync.WaitGroup
	logger            *slog.Logger
	connCountMutex    *sync.RWMutex
	activeConns       *sync.Map
	ListenAddr        string `mapstructure:"ListenAddr"`
	addr              string
	InputTag          string        `mapstructure:"Tag"`
	Port              int           `mapstructure:"Port"`
	BufferSize        int64         `mapstructure:"BufferSize"`
	ConnectionTimeout time.Duration `mapstructure:"ConnectionTimeout"`
	connCount         int32
}

func ParseTCP(input map[string]interface{}, logger *slog.Logger) (InTCP, error) {
	tcpObject := InTCP{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.TextUnmarshallerHookFunc(),
		Result:     &tcpObject,
	})
	if err != nil {
		return tcpObject, fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(input); err != nil {
		return tcpObject, fmt.Errorf("failed to decode TCP config: %w", err)
	}

	tcpObject.BufferSize = tcpObject.BufferSize << 10 // Convert to KB

	// Set Defaults
	tcpObject.ListenAddr = cmp.Or(tcpObject.ListenAddr, "0.0.0.0")
	tcpObject.Port = cmp.Or(tcpObject.Port, 6666)
	tcpObject.BufferSize = cmp.Or(tcpObject.BufferSize, defaultTCPBufferSize)
	tcpObject.ConnectionTimeout = cmp.Or(tcpObject.ConnectionTimeout, defaultTCPTimeout)
	tcpObject.addr = fmt.Sprintf("%s:%d", tcpObject.ListenAddr, tcpObject.Port)
	tcpObject.ctx, tcpObject.cancel = context.WithCancel(context.Background())
	tcpObject.logger = logger
	tcpObject.sendCh = make(chan util.Event, 500) // Buffered channel to prevent blocking
	tcpObject.wg = &sync.WaitGroup{}
	tcpObject.connCountMutex = &sync.RWMutex{}
	tcpObject.activeConns = &sync.Map{}

	return tcpObject, nil
}

func (iTcp *InTCP) incrementConnCount() bool {
	iTcp.connCountMutex.Lock()
	defer iTcp.connCountMutex.Unlock()

	if iTcp.connCount >= maxConnectionCountTCP {
		return false
	}

	iTcp.connCount++
	return true
}

func (iTcp *InTCP) decrementConnCount() {
	iTcp.connCountMutex.Lock()
	defer iTcp.connCountMutex.Unlock()

	if iTcp.connCount > 0 {
		iTcp.connCount--
	}
}

func (iTcp *InTCP) handleConnection(connState *connState) {
	defer iTcp.wg.Done()
	defer connState.Close()
	defer iTcp.decrementConnCount()
	defer iTcp.activeConns.Delete(connState)

	conn := connState.conn
	remoteAddr := conn.RemoteAddr().String()

	// Set initial deadline
	if err := conn.SetDeadline(time.Now().Add(iTcp.ConnectionTimeout * time.Minute)); err != nil {
		iTcp.logger.Error("Failed to set connection deadline", "error", err, "remote_addr", remoteAddr)
		return
	}

	buffer := make([]byte, iTcp.BufferSize)
	iTcp.logger.Info("New connection established", "remote_addr", remoteAddr)

	readCtx, cancel := context.WithCancel(iTcp.ctx)
	defer cancel()

	// Handle context cancellation
	go func() {
		<-readCtx.Done()
		if err := connState.Close(); err != nil {
			iTcp.logger.Debug("Error closing connection during shutdown",
				"error", err,
				"remote_addr", remoteAddr)
		}
	}()

	for {
		if connState.IsClosed() {
			return
		}

		select {
		case <-readCtx.Done():
			iTcp.logger.Info("Connection closed due to shutdown", "remote_addr", remoteAddr)
			return
		default:
			// Set a shorter read deadline to allow for quicker shutdown
			if err := conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
				if !connState.IsClosed() {
					iTcp.logger.Error("Failed to reset deadline",
						"error", err,
						"remote_addr", remoteAddr)
				}
				return
			}

			n, err := conn.Read(buffer)
			if err != nil {
				if err == io.EOF {
					iTcp.logger.Info("Client closed connection", "remote_addr", remoteAddr)
					return
				}

				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					if readCtx.Err() != nil {
						return
					}
					continue
				}

				// Only log if connection wasn't deliberately closed
				if !connState.IsClosed() && iTcp.ctx.Err() != nil {
					iTcp.logger.Error("Error reading from connection",
						"error", err,
						"remote_addr", remoteAddr)
				}
				return
			}

			// Reset the full timeout after successful read
			if err := conn.SetReadDeadline(time.Now().Add(iTcp.ConnectionTimeout * time.Minute)); err != nil {
				if !connState.IsClosed() {
					iTcp.logger.Error("Failed to reset timeout deadline",
						"error", err,
						"remote_addr", remoteAddr)
				}
				return
			}

			if n > 0 {
				metadata := map[string]interface{}{
					"SourceIP": remoteAddr,
				}

				select {
				case iTcp.sendCh <- util.Event{
					RawData:     buffer[:n],
					Time:        time.Now().Unix(),
					InputTag:    iTcp.GetTag(),
					InputSource: "tcp",
					Metadata:    metadata,
				}:
				default:
					iTcp.logger.Warn("Event channel full, dropping message", "remote_addr", remoteAddr)
				}
			}
		}
	}
}

func (iTcp InTCP) GetTag() string {
	if iTcp.InputTag == "" {
		return "*"
	}
	return iTcp.InputTag
}

func (iTcp InTCP) Start() {
	go func() {
		var err error
		iTcp.listener, err = net.Listen("tcp", iTcp.addr)
		if err != nil {
			iTcp.logger.Error("Couldn't start tcp input", "error", err)
			return
		}

		iTcp.logger.Info("Starting TCP input",
			"addr", iTcp.addr,
			"buffer_size", iTcp.BufferSize,
			"timeout", iTcp.ConnectionTimeout,
			"max_connections", maxConnectionCountTCP,
		)

		for {
			select {
			case <-iTcp.ctx.Done():
				return
			default:
				conn, err := iTcp.listener.Accept()
				if err != nil {
					if iTcp.ctx.Err() != nil {
						iTcp.logger.Warn("Error accepting TCP input connection", "error", err)
					}
					continue
				}

				if !iTcp.incrementConnCount() {
					iTcp.logger.Warn("Maximum connection limit reached, rejecting connection",
						"remote_addr", conn.RemoteAddr().String())
					conn.Close()
					continue
				}

				cs := newConnState(conn)
				iTcp.activeConns.Store(conn, struct{}{})
				iTcp.wg.Add(1)
				go iTcp.handleConnection(cs)
			}
		}
	}()
}

func (iTcp InTCP) Read() <-chan util.Event {
	return iTcp.sendCh
}

func (iTcp InTCP) Stop() {
	if iTcp.ctx != nil {
		iTcp.cancel()
	}

	if iTcp.listener != nil {
		if err := iTcp.listener.Close(); err != nil {
			iTcp.logger.Error("Error closing listener", "error", err)
		}
	}

	// Close all active connections immediately
	iTcp.activeConns.Range(func(key, value interface{}) bool {
		if conn, ok := key.(net.Conn); ok {
			conn.Close()
		}
		return true
	})

	iTcp.wg.Wait()
	close(iTcp.sendCh)
}
