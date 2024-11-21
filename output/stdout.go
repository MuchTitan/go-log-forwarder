package output

import (
	"encoding/json"
	"fmt"
	"log-forwarder-client/util"
	"os"

	"github.com/mitchellh/mapstructure"
)

type Stdout struct {
	OutputMatch string `mapstructure:"Match"`
	SendRaw     bool   `mapstructure:"SendRaw"`
}

func ParseStdout(input map[string]interface{}) (Stdout, error) {
	stdout := Stdout{}
	err := mapstructure.Decode(input, &stdout)
	if err != nil {
		return stdout, err
	}
	return stdout, nil
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

func (sto Stdout) GetMatch() string {
	if sto.OutputMatch == "" {
		return "*"
	}
	return sto.OutputMatch
}
