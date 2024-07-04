package reader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/nxadm/tail"
)

type Reader struct {
	path         string
	serverUrl    string
	cancelFunc   context.CancelFunc
	doneCh       chan struct{}
	Lines        []*LineData
	LastSendLine int
}

type LineData struct {
	FilePath  string `json:"filePath"`
	Data      string `json:"data"`
	Num       int    `json:"lineNumber"`
	Timestamp int64  `json:"timestamp"`
}

type Config struct {
	Path      string
	ServerUrl string
}

func (r *Reader) GetPath() string {
	return r.path
}

func (r *Reader) IsRunning() bool {
	return r.doneCh != nil
}

func createPostData(path string) *LineData {
	return &LineData{
		FilePath:  path,
		Timestamp: time.Now().Unix(),
	}
}

func postData(url string, data []byte) (bool, error) {
	res, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return false, err
	} else if res.StatusCode != http.StatusOK {
		return false, errors.New(fmt.Sprintf("While transmitting line %s the StatusCode was %d", string(data), res.StatusCode))
	}
	return true, nil
}

func New(config Config) *Reader {
	if _, err := os.Stat(config.Path); err != nil {
		log.Fatalf("No File found for path: %s", config.Path)
	}
	reader := &Reader{
		path:      config.Path,
		serverUrl: "http://127.0.0.1:8000/test",
		Lines:     []*LineData{},
	}

	if config.ServerUrl != "" {
		reader.serverUrl = config.ServerUrl
	}

	return reader
}

func (r *Reader) Start() {
	if r.doneCh != nil {
		return
	}
	t, err := tail.TailFile(r.path, tail.Config{Follow: true, ReOpen: true})
	if err != nil {
		log.Fatalln(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancelFunc = cancel

	done := make(chan struct{})
	r.doneCh = done
	defer close(done)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-t.Lines:
				if !ok {
					log.Fatalf("[ERROR] line read: %s", err)
					return
				}
				data := createPostData(r.path)
				data.Data, data.Num = line.Text, line.Num
				jsonData, err := json.Marshal(data)
				if err != nil {
					log.Printf("While processing data for Line %d in File %s an error occurred:\n%s", line.Num, r.path, err)
				}
				transmissionStatus, err := postData(r.serverUrl, jsonData)
				if err != nil {
					log.Println(err)
				}
				if transmissionStatus {
					r.LastSendLine = data.Num
				}
				r.Lines = append(r.Lines, data)
			}
		}
	}()
}

func (r *Reader) Stop() {
	if r.doneCh == nil {
		return
	}
	fmt.Printf("Stopping reader for File: %s\n", r.path)
	r.cancelFunc()
	<-r.doneCh
}
