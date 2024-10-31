package input

import (
	"cmp"
	"context"
	"fmt"
	"log-forwarder-client/util"
	"log/slog"
	"net"
	"time"

	"github.com/mitchellh/mapstructure"
)

const (
	defaultUDPBufferSize int64 = 64 << 10 // 64KB
)

type InUDP struct {
	addr       string
	InputTag   string `mapstructure:"Tag"`
	ListenAddr string `mapstructure:"ListenAddr"`
	Port       int    `mapstructure:"Port"`
	BufferSize int64  `mapstructure:"BufferSize"`
	ctx        context.Context
	cancel     context.CancelFunc
	sendCh     chan util.Event
	listener   *net.UDPConn
	logger     *slog.Logger
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

	// Set defaults
	udpObject.ListenAddr = cmp.Or(udpObject.ListenAddr, "0.0.0.0")
	udpObject.Port = cmp.Or(udpObject.Port, 6666)
	udpObject.BufferSize = cmp.Or(udpObject.BufferSize, defaultUDPBufferSize)
	udpObject.addr = fmt.Sprintf("%s:%d", udpObject.ListenAddr, udpObject.Port)
	udpObject.ctx, udpObject.cancel = context.WithCancel(context.Background())
	udpObject.logger = logger
	udpObject.sendCh = make(chan util.Event, 500) // Buffered channel to prevent blocking

	return udpObject, nil
}

func (iUdp InUDP) Start() {
	go func() {
		var err error
		iUdp.listener, err = net.ListenUDP("udp", &net.UDPAddr{
			IP:   net.ParseIP(iUdp.ListenAddr),
			Port: iUdp.Port,
		})
		if err != nil {
			iUdp.logger.Error("Couldn't start UDP input", "error", err)
			return
		}

		iUdp.logger.Info("Starting UDP input",
			"addr", iUdp.addr,
			"buffer_size", iUdp.BufferSize,
		)

		buffer := make([]byte, iUdp.BufferSize)

		for {
			select {
			case <-iUdp.ctx.Done():
				if err := iUdp.listener.Close(); err != nil {
					iUdp.logger.Error("Error closing listener", "error", err)
				}
				return
			default:
				// Read from the UDP listener
				n, remoteAddr, err := iUdp.listener.ReadFromUDP(buffer)
				if err != nil {
					iUdp.logger.Error("Error reading from UDP connection", "error", err)
					continue
				}

				if n > 0 {
					metadata := map[string]interface{}{
						"SourceIP": remoteAddr.String(),
					}

					// Send the event to the channel
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
	}()
}

func (iUdp InUDP) Read() <-chan util.Event {
	return iUdp.sendCh
}

func (iUdp InUDP) Stop() {
	if iUdp.ctx != nil {
		iUdp.cancel()
	}

	close(iUdp.sendCh)
}

func (iUdp InUDP) GetTag() string {
	if iUdp.InputTag == "" {
		return "*"
	}
	return iUdp.InputTag
}
