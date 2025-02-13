package util

import (
	"fmt"
	"strings"
)

func TagMatch(inputTag, match string) bool {
	// Split the pattern by '*' and get the parts.
	if match == "" && inputTag != "" {
		return false
	}
	parts := strings.Split(match, "*")

	// Keep track of the current position in the input string.
	pos := 0

	for i, part := range parts {
		if part == "" {
			continue
		}

		// If it's the first part, the input string must start with this part.
		if i == 0 && !strings.HasPrefix(inputTag, part) {
			return false
		}

		// If it's the last part, the input string must end with this part.
		if i == len(parts)-1 && !strings.HasSuffix(inputTag, part) {
			return false
		}

		// Find the next occurrence of the part in the input string starting from `pos`.
		index := strings.Index(inputTag[pos:], part)
		if index == -1 {
			return false
		}

		// Move the position forward.
		pos += index + len(part)
	}

	return true
}

func MergeMaps(m1, m2 map[string]interface{}) map[string]interface{} {
	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}

func MustString(data interface{}) string {
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
