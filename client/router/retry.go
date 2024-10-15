package router

import (
	"log-forwarder-client/output"
	"log-forwarder-client/parser"
	"log/slog"
	"sync"
	"time"
)

type RetryData struct {
	LineData parser.ParsedData
	Outputs  []output.Output
}

type RetryQueue struct {
	logger *slog.Logger
	doneCh chan struct{}
	data   []*RetryData
	mu     sync.Mutex
}

func NewRetryQueue(logger *slog.Logger) *RetryQueue {
	return &RetryQueue{
		doneCh: make(chan struct{}),
		data:   []*RetryData{},
		logger: logger,
	}
}

func (rq *RetryQueue) AddRetryData(data parser.ParsedData, outputs []output.Output) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	rd := &RetryData{
		LineData: data,
		Outputs:  outputs,
	}
	rq.data = append(rq.data, rd)
}

func (rq *RetryQueue) RetryHandlerLoop() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rq.mu.Lock()
			if len(rq.data) > 15 {
				rq.logger.Warn("More than 15 elements in RetryQueue", "amount", len(rq.data))
			}

			var remainingData []*RetryData
			for _, data := range rq.data {
				allSucceeded := true
				var outputs []output.Output
				for _, output := range data.Outputs {
					err := output.Write(data.LineData)
					if err != nil {
						allSucceeded = false
						outputs = append(outputs, output)
					}
				}

				if !allSucceeded {
					data.Outputs = outputs
					remainingData = append(remainingData, data)
				}
			}

			// Update the data slice with only remaining data
			rq.data = remainingData
			rq.mu.Unlock()

		case <-rq.doneCh:
			return
		}
	}
}

func (rq *RetryQueue) Stop() {
	close(rq.doneCh)
}

func (rq *RetryQueue) GetState() []*RetryData {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	return rq.data
}

func allOutputsSucceeded(outputs map[output.Output]bool) bool {
	for _, succeeded := range outputs {
		if !succeeded {
			return false
		}
	}
	return true
}
