package output

var ValidOutputs = []string{"Splunk", "PostgreSQL"}

type DefiendOutputs struct{}

type Output interface {
	Filter() []byte
	Send() bool
}
