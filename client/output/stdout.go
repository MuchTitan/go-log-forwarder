package output

import (
	"encoding/json"
	"fmt"
	"log-forwarder-client/parser"
	"log-forwarder-client/utils"
	"os"
)

type Stdout struct {
	Name string
}

func NewStout() Stdout {
	return Stdout{
		Name: "stdout",
	}
}

func (st Stdout) Write(data parser.ParsedData) error {
	dataWithMetadata := utils.MergeMaps(data.Data, data.Metadata)
	byteData, _ := json.Marshal(dataWithMetadata)
	_, err := fmt.Fprintln(os.Stdout, string(byteData))
	return err
}
