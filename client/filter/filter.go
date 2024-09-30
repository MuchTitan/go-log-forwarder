package filter

import "log-forwarder-client/parser"

type Filter interface {
	Apply(data parser.ParsedData) (parser.ParsedData, bool)
}
