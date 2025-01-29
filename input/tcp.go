package input

import (
	"context"
	"errors"
	"fmt"
	"io"
	"github.com/MuchTitan/go-log-forwarder/global"
	"github.com/MuchTitan/go-log-forwarder/util"
	"log/slog"
	"net"
	"sync"
	"time"
)

const (
	defaultTCPBufferSize  = 64 << 10 // 64KB
	defaultTCPTimeout     = 10       // 10 minute timeout
	maxConnectionCountTCP = 50       // Maximum number of concurrent connections
)

type TCP struct {
	name           string
	tag            string
	listenAddr     string
	port           int
	bufferSize     int64
	timeout        time.Duration
	listener       net.Listener
	activeConns    sync.Map
	connCount      int32
	connCountMutex sync.RWMutex
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
}

type connState struct {
	conn     net.Conn
	closed   bool
	closeMux sync.Mutex
}

func newConnState(conn net.Conn) *connState {
	return &connState{
		conn:   conn,
		closed: false,
	}
}

func (cs *connState) Close() error {
	cs.closeMux.Lock()
	defer cs.closeMux.Unlock()

	if !cs.closed {
		cs.closed = true
		return cs.conn.Close()
	}
	return nil
}

func (cs *connState) IsClosed() bool {
	cs.closeMux.Lock()
	defer cs.closeMux.Unlock()
	return cs.closed
}

func (t *TCP) Name() string {
	return t.name
}

func (t *TCP) Tag() string {
	return t.tag
}

func (t *TCP) Init(config map[string]interface{}) error {
	t.listenAddr = util.MustString(config["ListenAddr"])
	if t.listenAddr == "" {
		t.listenAddr = "0.0.0.0"
	}

	if portStr, exists := config["Port"]; exists {
		var ok bool
		if t.port, ok = portStr.(int); !ok {
			return errors.New("cant convert port to int")
		}
	} else {
		t.port = 6666
	}

	if bufferSizeStr, exists := config["BufferSize"]; exists {
		var ok bool
		if t.bufferSize, ok = bufferSizeStr.(int64); !ok {
			return errors.New("cant convert bufferSize to int")
		}
		t.bufferSize <<= 10 // Convert to KB
	} else {
		t.bufferSize = defaultTCPBufferSize
	}

	if timeoutStr, exists := config["Timeout"]; exists {
		if _, err := fmt.Sscanf(timeoutStr.(string), "%d", &t.timeout); err != nil {
			return fmt.Errorf("invalid timeout: %v", err)
		}
	} else {
		t.timeout = defaultTCPTimeout
	}

	t.name = util.MustString(config["Name"])
	if t.name == "" {
		t.name = "tcp"
	}

	t.tag = util.MustString(config["Tag"])
	if t.tag == "" {
		t.tag = "tcp"
	}

	return nil
}

func (t *TCP) incrementConnCount() bool {
	t.connCountMutex.Lock()
	defer t.connCountMutex.Unlock()

	if t.connCount >= maxConnectionCountTCP {
		return false
	}

	t.connCount++
	return true
}

func (t *TCP) decrementConnCount() {
	t.connCountMutex.Lock()
	defer t.connCountMutex.Unlock()

	if t.connCount > 0 {
		t.connCount--
	}
}

func (t *TCP) handleConnection(cs *connState, output chan<- global.Event) {
	defer t.wg.Done()
	defer cs.Close()
	defer t.decrementConnCount()
	defer t.activeConns.Delete(cs)

	conn := cs.conn
	remoteAddr := conn.RemoteAddr().String()
	linenumber := 0

	if err := conn.SetDeadline(time.Now().Add(t.timeout * time.Minute)); err != nil {
		slog.Error("Failed to set connection deadline", "error", err, "remote_addr", remoteAddr)
		return
	}

	buffer := make([]byte, t.bufferSize)
	slog.Info("New connection established", "remote_addr", remoteAddr)

	readCtx, cancel := context.WithCancel(t.ctx)
	defer cancel()

	go func() {
		<-readCtx.Done()
		if err := cs.Close(); err != nil {
			slog.Debug("Error closing connection during shutdown",
				"error", err,
				"remote_addr", remoteAddr)
		}
	}()

	for {
		if cs.IsClosed() {
			return
		}

		select {
		case <-readCtx.Done():
			slog.Info("[TCP] Connection closed due to shutdown", "remote_addr", remoteAddr)
			return
		default:
			if err := conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
				if !cs.IsClosed() {
					slog.Error("[TCP] Failed to reset deadline",
						"error", err,
						"remote_addr", remoteAddr)
				}
				return
			}

			n, err := conn.Read(buffer)
			if err != nil {
				if err == io.EOF {
					slog.Info("Client closed connection", "remote_addr", remoteAddr)
					return
				}

				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					if readCtx.Err() != nil {
						return
					}
					continue
				}

				if !cs.IsClosed() && t.ctx.Err() != nil {
					slog.Error("[TCP] Error reading from connection",
						"error", err,
						"remote_addr", remoteAddr)
				}
				return
			}

			if err := conn.SetReadDeadline(time.Now().Add(t.timeout * time.Minute)); err != nil {
				if !cs.IsClosed() {
					slog.Error("[TCP] Failed to reset timeout deadline",
						"error", err,
						"remote_addr", remoteAddr)
				}
				return
			}

			if n > 0 {
				linenumber++
				event := global.Event{
					Timestamp: time.Now(),
					RawData:   string(buffer[:n]),
					Metadata: global.Metadata{
						Source:  remoteAddr,
						LineNum: linenumber,
					},
				}
				AddMetadata(&event, t)

				select {
				case output <- event:
				case <-t.ctx.Done():
					return
				default:
					slog.Warn("[TCP] Event channel full, dropping message", "remote_addr", remoteAddr)
				}
			}
		}
	}
}

func (t *TCP) Start(parentCtx context.Context, output chan<- global.Event) error {
	var err error
	addr := fmt.Sprintf("%s:%d", t.listenAddr, t.port)
	t.ctx, t.cancel = context.WithCancel(parentCtx)

	t.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("couldn't start tcp input: %w", err)
	}

	slog.Info("[TCP] Starting",
		"addr", addr,
		"buffer_size", t.bufferSize,
		"timeout", t.timeout,
		"max_connections", maxConnectionCountTCP,
	)

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			select {
			case <-t.ctx.Done():
				return
			default:
				conn, err := t.listener.Accept()
				if err != nil {
					if t.ctx.Err() != nil {
						slog.Warn("[TCP] Error accepting input connection", "error", err)
					}
					continue
				}

				if !t.incrementConnCount() {
					slog.Warn("[TCP] Maximum connection limit reached, rejecting connection",
						"remote_addr", conn.RemoteAddr().String())
					conn.Close()
					continue
				}

				cs := newConnState(conn)
				t.activeConns.Store(cs, struct{}{})
				t.wg.Add(1)
				go t.handleConnection(cs, output)
			}
		}
	}()

	return nil
}

func (t *TCP) Exit() error {
	slog.Info("[TCP] Stopping")
	if t.cancel != nil {
		t.cancel()
	}

	if t.listener != nil {
		if err := t.listener.Close(); err != nil {
			slog.Error("[TCP] Error closing listener", "error", err)
		}
	}

	t.activeConns.Range(func(key, value interface{}) bool {
		if cs, ok := key.(*connState); ok {
			cs.Close()
		}
		return true
	})

	t.wg.Wait()
	return nil
}
