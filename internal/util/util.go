package util

import (
	"fmt"
	"maps"
	"regexp"
	"strings"
)

func GlobMatch(input, match string) bool {
	regexPattern := "^" + regexp.QuoteMeta(match)
	regexPattern = strings.ReplaceAll(regexPattern, "\\*", ".*")
	regexPattern = strings.ReplaceAll(regexPattern, "\\?", ".")
	regexPattern += "$"

	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return false
	}

	if regex.MatchString(input) {
		return true
	}

	return false
}

func MergeMaps(m1, m2 map[string]any) map[string]any {
	maps.Copy(m1, m2)
	return m1
}

func MustString(data any) string {
	if data == nil {
		return ""
	}
	var stringData string
	var ok bool
	if stringData, ok = data.(string); !ok {
		fmt.Println(data)
		panic("Cant convert interface to string")
	}
	return stringData
}
