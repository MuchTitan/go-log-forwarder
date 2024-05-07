package reader

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"log-forwarder-client/config"
	"log-forwarder-client/models"
	"net/http"

	"github.com/nxadm/tail"
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
      
      res, err := http.Post(url,"application/json",bytes.NewBuffer([]byte(logline.Data)))
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
