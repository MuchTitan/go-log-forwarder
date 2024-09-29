package filter

type Filter interface {
	Apply(data map[string]interface{}) (map[string]interface{}, bool)
}
