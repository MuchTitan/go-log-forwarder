package output

import (
	"encoding/json"
	"fmt"
	"os"
)

type Stdout struct{}

func (st Stdout) Write(data map[string]interface{}) {
	byteData, _ := json.Marshal(data)
	fmt.Fprintln(os.Stdout, string(byteData))
}
