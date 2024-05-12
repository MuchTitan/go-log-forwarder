package reader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log-forwarder-client/config"
	"log-forwarder-client/models"
	"net/http"
	"time"

	"github.com/nxadm/tail"
)

type PostData struct {
  FilePath string `json:"filePath"`
  Data string `json:"data"`
  Num int `json:"lineNumber"`
  Timestamp int64 `json:"timestamp"`
}

func createPostData(path string) *PostData {
  return &PostData{FilePath: path ,Timestamp: time.Now().Unix()}
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
				return
			case line, ok := <-t.Lines:
				if !ok {
					return
				}
				result := models.LogLine{Data: line.Text,LineNum: line.Num, TransmitionStatus: false}
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

func ReadFile(ctx context.Context, path string) error {
  readerChannel := Reader(ctx,path)
	url := fmt.Sprintf("http://%s:%s/test", config.Env("ServerUrl"), config.Env("ListenPort"))
  for {
    select {
    case <-ctx.Done():
      return ctx.Err()
    case logline, ok := <- readerChannel:
      if !ok {
        return nil
      }
      data := createPostData(path) 
      data.Data = logline.Data
      data.Num = logline.LineNum
      json_data, err := json.Marshal(data)
      if err != nil {
        log.Printf("Failed to create PostData for LogLine %s",logline.Data)
      }
      res, err := http.Post(url,"application/json",bytes.NewBuffer(json_data))
      if err != nil {
        logline.TransmitionStatus = false
        log.Println(err)
      } else if res.StatusCode != http.StatusOK {
				log.Printf("While transmitting line %s the StatusCode was %d", logline.Data, res.StatusCode)
        logline.TransmitionStatus = false
      } else {
        logline.TransmitionStatus = true
      } 
    }
  }
}
