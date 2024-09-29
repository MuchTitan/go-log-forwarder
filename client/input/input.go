package input

var ValidInputs = []string{"syslog"}

type Input interface {
	Read() <-chan string
	Stop()
}
