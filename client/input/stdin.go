package input

import (
	"bufio"
	"context"
	"io"
	"log-forwarder-client/util"
	"log/slog"
	"os"
	"strings"
	"time"
)

type StdIn struct {
	ctx      context.Context
	logger   *slog.Logger
	cancel   context.CancelFunc
	sendCh   chan util.Event
	doneCh   chan struct{}
	lineCh   chan string
	inputTag string
}

func NewStdIn(inputTag string, logger *slog.Logger) StdIn {
	ctx, cancel := context.WithCancel(context.Background())
	return StdIn{
		inputTag: inputTag,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		sendCh:   make(chan util.Event),
		doneCh:   make(chan struct{}),
		lineCh:   make(chan string, 10),
	}
}

func (stdin StdIn) ReadFromStdIn() {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				continue
			}
			stdin.logger.Error("Couldn't read line from StdIn", "error", err)
			return
		}
		stdin.lineCh <- strings.TrimSuffix(line, "\n")
	}
}

func (stdin StdIn) Start() {
	go stdin.ReadFromStdIn()

	go func() {
		for {
			select {
			case <-stdin.ctx.Done():
				close(stdin.doneCh)
				return
			case line, ok := <-stdin.lineCh:
				if !ok {
					stdin.logger.Warn("Read from stdin ended unexpected")
					return
				}
				out := util.Event{}
				out.RawData = []byte(line)
				out.Time = time.Now().Unix()
				out.InputTag = stdin.inputTag
				stdin.sendCh <- out
			}
		}
	}()
}

func (stdin StdIn) Stop() {
	if stdin.ctx != nil {
		stdin.cancel()
		<-stdin.doneCh
		close(stdin.sendCh)
	}
	stdin.logger.Debug("Stopping read from StdIn")
}

func (stdin StdIn) Read() <-chan util.Event {
	return stdin.sendCh
}
