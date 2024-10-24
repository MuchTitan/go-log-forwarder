package output

import "log-forwarder-client/util"

var AvailableOutputs []Output

type Output interface {
	GetMatch() string
	Write(util.Event) error
}
