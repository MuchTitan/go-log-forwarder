package parser

type Parser interface {
	Apply(data string) (map[string]interface{}, error)
}
