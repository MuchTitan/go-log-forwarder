package filter

import "log-forwarder-client/util"

var AvailableFilters []Filter

type Filter interface {
	GetMatch() string
	Apply(*util.Event) bool
}
