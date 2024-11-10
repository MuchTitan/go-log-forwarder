package filter

import (
	"fmt"
	"log-forwarder-client/util"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUtil(t *testing.T) {
	t.Run("TestGrep", func(t *testing.T) {
		currentTime := time.Now().Unix()
		testData := []struct {
			name   string
			g      Grep
			data   *util.Event
			expect bool
		}{
			{
				name: "Empty grep should return false",
				g:    Grep{},
				data: &util.Event{
					ParsedData: map[string]interface{}{"message": "test log"},
					Time:       currentTime,
				},
				expect: false,
			},
			{
				name: "OR operation with single matching regex",
				g: Grep{
					Op:    "or",
					Regex: []string{"test"},
				},
				data: &util.Event{
					ParsedData: map[string]interface{}{"message": "test log"},
					Time:       currentTime,
				},
				expect: true,
			},
			{
				name: "OR operation with no matching regex",
				g: Grep{
					Op:    "or",
					Regex: []string{"foo", "bar"},
				},
				data: &util.Event{
					ParsedData: map[string]interface{}{"message": "test log"},
					Time:       currentTime,
				},
				expect: false,
			},
			{
				name: "AND operation with all matching regex",
				g: Grep{
					Op:    "and",
					Regex: []string{"test", "log"},
				},
				data: &util.Event{
					ParsedData: map[string]interface{}{"message": "test log"},
					Time:       currentTime,
				},
				expect: true,
			},
			{
				name: "AND operation with partial matching regex",
				g: Grep{
					Op:    "and",
					Regex: []string{"test", "missing"},
				},
				data: &util.Event{
					ParsedData: map[string]interface{}{"message": "test log"},
					Time:       currentTime,
				},
				expect: false,
			},
			{
				name: "Complex nested data with regex",
				g: Grep{
					Op:    "and",
					Regex: []string{`"level":"error"`, `"code":500`},
				},
				data: &util.Event{
					ParsedData: map[string]interface{}{
						"level": "error",
						"code":  500,
						"details": map[string]interface{}{
							"message": "internal server error",
						},
					},
					Time: currentTime,
				},
				expect: true,
			},
			{
				name: "Test with exclude patterns",
				g: Grep{
					Op:      "and",
					Regex:   []string{"error"},
					Exclude: []string{"timeout"},
				},
				data: &util.Event{
					ParsedData: map[string]interface{}{
						"message": "database error occurred",
					},
					Time: currentTime,
				},
				expect: false,
			},
			{
				name: "Test FilterMatch getter with empty value",
				g: Grep{
					FilterMatch: "",
				},
				data: &util.Event{
					ParsedData: map[string]interface{}{
						"message": "test",
					},
					Time: currentTime,
				},
				expect: false,
			},
			{
				name: "Test FilterMatch getter with custom value",
				g: Grep{
					FilterMatch: "custom_match",
				},
				data: &util.Event{
					ParsedData: map[string]interface{}{
						"message": "test",
					},
					Time: currentTime,
				},
				expect: false,
			},
		}

		for _, test := range testData {
			t.Run(test.name, func(t *testing.T) {
				result := test.g.Apply(test.data)
				assert.Equal(t, test.expect, result, fmt.Sprintf("Failed test case: %s\nLogicOp: %s, Regex: %v, Exclude: %v",
					test.name, test.g.Op, test.g.Regex, test.g.Exclude))
			})
		}
	})
}
