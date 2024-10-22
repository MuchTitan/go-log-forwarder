package output

import "log-forwarder-client/util"

var ValidOutputs = []string{"splunk", "stdout"}

type Output interface {
	Write(util.Event) error
}
