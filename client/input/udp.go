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
	defaultUDPBufferSize  int64 = 64 << 10         // 64KB
	defaultUDPTimeout           = 10 * time.Minute // 10 minute timeout
	maxConnectionCountUDP       = 100              // Maximum number of concurrent connections
)

type InUDP struct {
	addr              string
	InputTag          string        `mapstructure:"Tag"`
	ListenAddr        string        `mapstructure:"ListenAddr"`
	Port              int           `mapstructure:"Port"`
	BufferSize        int64         `mapstructure:"BufferSize"`
	ConnectionTimeout time.Duration `mapstructure:"ConnectionTimeout"`
	ctx               context.Context
	cancel            context.CancelFunc
	sendCh            chan util.Event
	listener          net.Listener
	connCount         int32
	logger            *slog.Logger
	wg                *sync.WaitGroup
	connCountMutex    *sync.RWMutex
	activeConns       *sync.Map
}

func ParseUDP(input map[string]interface{}, logger *slog.Logger) (InUDP, error) {
	udpObject := InUDP{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.TextUnmarshallerHookFunc(),
		Result:     &udpObject,
	})
	if err != nil {
		return udpObject, fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(input); err != nil {
		return udpObject, fmt.Errorf("failed to decode UDP config: %w", err)
	}

	udpObject.BufferSize = udpObject.BufferSize << 10 // Convert to KB

	// Set Defaults
	udpObject.ListenAddr = cmp.Or(udpObject.ListenAddr, "0.0.0.0")
	udpObject.Port = cmp.Or(udpObject.Port, 6666)
	udpObject.BufferSize = cmp.Or(udpObject.BufferSize, defaultUDPBufferSize)
	udpObject.ConnectionTimeout = cmp.Or(udpObject.ConnectionTimeout, defaultUDPTimeout)
	udpObject.addr = fmt.Sprintf("%s:%d", udpObject.ListenAddr, udpObject.Port)
	udpObject.ctx, udpObject.cancel = context.WithCancel(context.Background())
	udpObject.logger = logger
	udpObject.sendCh = make(chan util.Event, 500) // Buffered channel to prevent blocking
	udpObject.wg = &sync.WaitGroup{}
	udpObject.connCountMutex = &sync.RWMutex{}
	udpObject.activeConns = &sync.Map{}

	return udpObject, nil
}

func (iUdp *InUDP) incrementConnCount() bool {
	iUdp.connCountMutex.Lock()
	defer iUdp.connCountMutex.Unlock()

	if iUdp.connCount >= maxConnectionCountUDP {
		return false
	}

	iUdp.connCount++
	return true
}

func (iUdp *InUDP) decrementConnCount() {
	iUdp.connCountMutex.Lock()
	defer iUdp.connCountMutex.Unlock()

	if iUdp.connCount > 0 {
		iUdp.connCount--
	}
}

func (iUdp *InUDP) handleConnection(connState *connState) {
	defer iUdp.wg.Done()
	defer connState.Close()
	defer iUdp.decrementConnCount()
	defer iUdp.activeConns.Delete(connState)

	conn := connState.conn
	remoteAddr := conn.RemoteAddr().String()

	// Set initial deadline
	if err := conn.SetDeadline(time.Now().Add(iUdp.ConnectionTimeout)); err != nil {
		iUdp.logger.Error("Failed to set connection deadline", "error", err, "remote_addr", remoteAddr)
		return
	}

	buffer := make([]byte, iUdp.BufferSize)
	iUdp.logger.Info("New connection established", "remote_addr", remoteAddr)

	readCtx, cancel := context.WithCancel(iUdp.ctx)
	defer cancel()

	// Handle context cancellation
	go func() {
		<-readCtx.Done()
		if err := connState.Close(); err != nil {
			iUdp.logger.Debug("Error closing connection during shutdown",
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
			iUdp.logger.Info("Connection closed due to shutdown", "remote_addr", remoteAddr)
			return
		default:
			// Set a shorter read deadline to allow for quicker shutdown
			if err := conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
				if !connState.IsClosed() {
					iUdp.logger.Error("Failed to reset deadline",
						"error", err,
						"remote_addr", remoteAddr)
				}
				return
			}

			n, err := conn.Read(buffer)
			if err != nil {
				if err == io.EOF {
					iUdp.logger.Info("Client closed connection", "remote_addr", remoteAddr)
					return
				}

				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					if readCtx.Err() != nil {
						return
					}
					continue
				}

				// Only log if connection wasn't deliberately closed
				if !connState.IsClosed() && iUdp.ctx.Err() != nil {
					iUdp.logger.Error("Error reading from connection",
						"error", err,
						"remote_addr", remoteAddr)
				}
				return
			}

			// Reset the full timeout after successful read
			if err := conn.SetReadDeadline(time.Now().Add(iUdp.ConnectionTimeout)); err != nil {
				if !connState.IsClosed() {
					iUdp.logger.Error("Failed to reset timeout deadline",
						"error", err,
						"remote_addr", remoteAddr)
				}
				return
			}

			metadata := map[string]interface{}{
				"SourceIP": remoteAddr,
			}

			select {
			case iUdp.sendCh <- util.Event{
				RawData:  buffer[:n],
				Time:     time.Now().Unix(),
				InputTag: iUdp.GetTag(),
				Metadata: metadata,
			}:
			default:
				iUdp.logger.Warn("Event channel full, dropping message", "remote_addr", remoteAddr)
			}
		}
	}
}

func (iUdp InUDP) GetTag() string {
	if iUdp.InputTag == "" {
		return "*"
	}
	return iUdp.InputTag
}

func (iUdp InUDP) Start() {
	go func() {
		var err error
		iUdp.listener, err = net.Listen("udp", iUdp.addr)
		if err != nil {
			iUdp.logger.Error("Couldn't start upd input", "error", err)
			return
		}

		iUdp.logger.Info("Starting UDP input",
			"addr", iUdp.addr,
			"buffer_size", iUdp.BufferSize,
			"timeout", iUdp.ConnectionTimeout,
			"max_connections", maxConnectionCountUDP,
		)

		for {
			select {
			case <-iUdp.ctx.Done():
				return
			default:
				conn, err := iUdp.listener.Accept()
				if err != nil {
					if iUdp.ctx.Err() != nil {
						iUdp.logger.Warn("Error accepting UDP input connection", "error", err)
					}
					continue
				}

				if !iUdp.incrementConnCount() {
					iUdp.logger.Warn("Maximum connection limit reached, rejecting connection",
						"remote_addr", conn.RemoteAddr().String())
					conn.Close()
					continue
				}

				cs := newConnState(conn)
				iUdp.activeConns.Store(conn, struct{}{})
				iUdp.wg.Add(1)
				go iUdp.handleConnection(cs)
			}
		}
	}()
}

func (iUdp InUDP) Read() <-chan util.Event {
	return iUdp.sendCh
}

func (iUdp InUDP) Stop() {
	if iUdp.ctx != nil {
		iUdp.cancel()
	}

	if iUdp.listener != nil {
		if err := iUdp.listener.Close(); err != nil {
			iUdp.logger.Error("Error closing listener", "error", err)
		}
	}

	// Close all active connections immediately
	iUdp.activeConns.Range(func(key, value interface{}) bool {
		if conn, ok := key.(net.Conn); ok {
			conn.Close()
		}
		return true
	})

	iUdp.wg.Wait()
	close(iUdp.sendCh)
}
