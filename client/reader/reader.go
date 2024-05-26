package reader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log-forwarder-client/config"
	"log-forwarder-client/models"
	"net/http"
	"time"

	"github.com/nxadm/tail"
)

type PostData struct {
	FilePath  string `json:"filePath"`
	Data      string `json:"data"`
	Num       int    `json:"lineNumber"`
	Timestamp int64  `json:"timestamp"`
}

func createPostData(path string) PostData {
	return PostData{FilePath: path, Timestamp: time.Now().Unix()}
}

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
				t.Kill(errors.New("Stopping the Tail because of Context"))
				return
			case line, ok := <-t.Lines:
				if !ok {
					return
				}
				result := models.LogLine{Data: line.Text, LineNum: line.Num, TransmitionStatus: false}
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

func ReadFile(file *models.FileState) error {
	readerChannel := Reader(file.State, file.Path)
	url := fmt.Sprintf("http://%s:%s/test", config.Env("ServerUrl"), config.Env("ListenPort"))
	for {
		select {
		case <-file.State.Done():
			return file.State.Err()
		case logline, ok := <-readerChannel:
			if !ok {
				return nil
			}
			data := createPostData(file.Path)
			data.Data = logline.Data
			data.Num = logline.LineNum
			json_data, err := json.Marshal(data)
			if err != nil {
				log.Printf("Failed to create PostData for LogLine %s\n%s", logline.Data, err)
			}
			res, err := http.Post(url, "application/json", bytes.NewBuffer(json_data))
			if err != nil {
				log.Println(err)
				logline.TransmitionStatus = false
			} else if res.StatusCode != http.StatusOK {
				log.Printf("While transmitting line %d in File %s the StatusCode was %d", logline.LineNum, file.Path, res.StatusCode)
				logline.TransmitionStatus = false
			} else {
				logline.TransmitionStatus = true
			}
			if logline.TransmitionStatus {
				file.LastSendLine = max(file.LastSendLine, logline.LineNum)
			}
			file.LogLines = append(file.LogLines, logline)
		}
	}
}
