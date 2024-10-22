package output

import (
	"encoding/json"
	"fmt"
	"log-forwarder-client/util"
	"os"
)

type Stdout struct {
	Name    string
	SendRaw bool
}

func NewStdout() Stdout {
	return Stdout{}
}

func (sto Stdout) Write(data util.Event) error {
	var event []byte
	if !sto.SendRaw {
		event, _ = json.Marshal(data.ParsedData)
	} else {
		event = data.RawData
	}
	_, err := fmt.Fprintln(os.Stdout, string(event))
	return err
}
