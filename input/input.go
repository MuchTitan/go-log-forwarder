package input

import (
	"log-forwarder-client/util"
	"net"
	"sync"
)

var AvailableInputs []Input

type Input interface {
	Start()
	GetTag() string
	Read() <-chan util.Event
	Stop()
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