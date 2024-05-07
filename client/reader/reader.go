package reader

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"github.com/nxadm/tail"
	"log-forwarder-client/config"
	"log-forwarder-client/models"
)

func Reader(ctx context.Context, path string) <-chan models.LogLine {
	out := make(chan models.LogLine)
	t, err := tail.TailFile(path, tail.Config{Follow: true, ReOpen: true})
	if err != nil {
		log.Fatalln(err)
	}
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-t.Lines:
				if !ok {
					return
				}
				result := models.LogLine{Data: line.Text,LineNum: line.Num, TransmitionStatus: false}
        log.Println(result)
				select {
				case <-ctx.Done():
					return
				case out <- result:
				}
			}
		}
	}()
	return out
}

func SendLineByLine(ctx context.Context, input <-chan string) <-chan bool {
	out := make(chan bool)
	url := fmt.Sprintf("http://%s:%s/test", config.Env("ServerUrl"), config.Env("ListenPort"))
	go func() {
    defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-input:
				if !ok {
					return
				}
				res, err := http.Post(url, "application/json", bytes.NewReader([]byte(line)))
				if err != nil {
					log.Println(err)
					out <- false
					continue
				}
				if res.StatusCode != http.StatusOK {
					log.Printf("While transmitting line %s the StatusCode was %d", line, res.StatusCode)
					out <- false
					continue
				}
				out <- true
			}
		}
	}()
	return out
}

func ReadFile(ctx context.Context, path string) error {

	readerChannel := Reader(ctx, path)
	lineChannel := make(chan string)
	resultChannel := make(chan bool)

	// Goroutine to send lines to be processed
	go func() {
		defer close(lineChannel)
		for logline := range readerChannel {
			lineChannel <- logline.Data
		}
	}()

	// Goroutine to receive results
	go func() {
		defer close(resultChannel)
		for result := range SendLineByLine(ctx, lineChannel) {
			resultChannel <- result
		}
	}()

	// Process results and update log lines
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case logline, ok := <-readerChannel:
			if !ok {
				return nil
			}
			result, ok := <-resultChannel
			if !ok {
				return nil
			}
			logline.TransmitionStatus = result
		}
	}
}
