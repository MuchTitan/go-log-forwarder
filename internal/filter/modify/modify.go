package modify

import (
	"errors"
	"fmt"
	"maps"
	"regexp"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/MuchTitan/go-log-forwarder/internal/util"
)

// conditions
const (
	KEYEXISTS         = "key_exists"
	KEYDOESNOTEXIST   = "no_key_exists"
	KEYMATCH          = "key_match"    // Uses Regex
	NOKEYMATCH        = "no_key_match" // Uses Regex
	KEYEQUALS         = "key_equals"
	KEYDOESNOTEQUAL   = "no_key_equal"
	VALUEEQUALS       = "value_equals"
	VALUEDOESNOTEQUAl = "no_value_equals"
)

type Modify struct {
	name           string
	match          string
	condition      map[string][]string // condition + array of key/value/regex
	set            map[string]string   // Overwrittes existing Key Value Pairs
	add            map[string]string   // Adds Key Value pair if key does not exist
	rename         map[string]string   // Rename key if exists and renamedKey does not exist
	hardRename     map[string]string   // Rename key if exists
	remove         []string            // Removes key value pair
	removeRegex    []regexp.Regexp     // Remove key value pair with Regex
	removeWildcard []string            // Remove key with wildcard support
}

func (m *Modify) Name() string {
	return m.name
}

func (m *Modify) MatchTag(inputTag string) bool {
	return util.GlobMatch(inputTag, m.match)
}

func (m *Modify) Type() internal.PluginType {
	return internal.FILTERMODIFY
}

func (m *Modify) Init(config map[string]any) error {
	m.name = util.MustString(config["Name"])
	if m.name == "" {
		m.name = "modify"
	}

	m.match = util.MustString(config["Match"])
	if m.match == "" {
		m.match = "*"
	}

	// Initialize maps
	m.set = make(map[string]string)
	m.add = make(map[string]string)
	m.rename = make(map[string]string)
	m.hardRename = make(map[string]string)
	m.condition = make(map[string][]string)
	m.remove = make([]string, 0)
	m.removeRegex = make([]regexp.Regexp, 0)
	m.removeWildcard = make([]string, 0)

	// Parse and validate condition
	if conditionConfig, exists := config["Condition"]; exists {
		if conditionMap, ok := conditionConfig.(map[string]any); ok {
			for k, v := range conditionMap {
				// Validate condition type
				switch k {
				case KEYEXISTS, KEYDOESNOTEXIST, KEYMATCH, NOKEYMATCH,
					KEYEQUALS, KEYDOESNOTEQUAL, VALUEEQUALS, VALUEDOESNOTEQUAl:
					// Handle both single values and arrays
					switch val := v.(type) {
					case string:
						m.condition[k] = []string{val}
					case []any:
						values := make([]string, 0, len(val))
						for _, item := range val {
							if strVal, ok := item.(string); ok {
								values = append(values, strVal)
							}
						}
						m.condition[k] = values
					default:
						return fmt.Errorf("invalid condition value type for '%s'", k)
					}
				default:
					return fmt.Errorf("invalid condition type '%s'", k)
				}
			}
		} else {
			return errors.New("condition must be a map")
		}
	} else {
		return errors.New("condition is required for modify filter")
	}

	// Parse Set operations
	if setConfig, exists := config["Set"]; exists {
		if setMap, ok := setConfig.(map[string]any); ok {
			for k, v := range setMap {
				if strVal, ok := v.(string); ok {
					m.set[k] = strVal
				}
			}
		}
	}

	// Parse Add operations
	if addConfig, exists := config["Add"]; exists {
		if addMap, ok := addConfig.(map[string]any); ok {
			for k, v := range addMap {
				if strVal, ok := v.(string); ok {
					m.add[k] = strVal
				}
			}
		}
	}

	// Parse Rename operations
	if renameConfig, exists := config["Rename"]; exists {
		if renameMap, ok := renameConfig.(map[string]any); ok {
			for k, v := range renameMap {
				if strVal, ok := v.(string); ok {
					m.rename[k] = strVal
				}
			}
		}
	}

	// Parse HardRename operations
	if hardRenameConfig, exists := config["HardRename"]; exists {
		if hardRenameMap, ok := hardRenameConfig.(map[string]any); ok {
			for k, v := range hardRenameMap {
				if strVal, ok := v.(string); ok {
					m.hardRename[k] = strVal
				}
			}
		}
	}

	// Parse Remove operations
	if removeConfig, exists := config["Remove"]; exists {
		if removeSlice, ok := removeConfig.([]any); ok {
			for _, v := range removeSlice {
				if strVal, ok := v.(string); ok {
					m.remove = append(m.remove, strVal)
				}
			}
		}
	}

	// Parse RemoveRegex operations
	if removeRegexConfig, exists := config["RemoveRegex"]; exists {
		if removeRegexSlice, ok := removeRegexConfig.([]any); ok {
			for _, v := range removeRegexSlice {
				if strVal, ok := v.(string); ok {
					regex, err := regexp.Compile(strVal)
					if err != nil {
						return fmt.Errorf("invalid regex pattern '%s': %v", strVal, err)
					}
					m.removeRegex = append(m.removeRegex, *regex)
				}
			}
		}
	}

	// Parse RemoveWildcard operations
	if removeWildcardConfig, exists := config["RemoveWildcard"]; exists {
		if removeWildcardSlice, ok := removeWildcardConfig.([]any); ok {
			for _, v := range removeWildcardSlice {
				if strVal, ok := v.(string); ok {
					m.removeWildcard = append(m.removeWildcard, strVal)
				}
			}
		}
	}

	// Validate that at least one operation is configured
	if len(m.set) == 0 && len(m.add) == 0 && len(m.rename) == 0 &&
		len(m.hardRename) == 0 && len(m.remove) == 0 &&
		len(m.removeRegex) == 0 && len(m.removeWildcard) == 0 {
		return errors.New("no modification operations configured for the modify filter")
	}

	return nil
}

func (m *Modify) Process(data *internal.Event) (*internal.Event, error) {
	if len(data.ParsedData) == 0 {
		return data, nil
	}

	// Check conditions first
	if len(m.condition) > 0 {
		conditionMet := false
		for conditionType, conditionValues := range m.condition {
			for _, conditionValue := range conditionValues {
				switch conditionType {
				case KEYEXISTS:
					if _, exists := data.ParsedData[conditionValue]; exists {
						conditionMet = true
					}
				case KEYDOESNOTEXIST:
					if _, exists := data.ParsedData[conditionValue]; !exists {
						conditionMet = true
					}
				case KEYMATCH:
					regex, err := regexp.Compile(conditionValue)
					if err != nil {
						continue
					}
					for key := range data.ParsedData {
						if regex.MatchString(key) {
							conditionMet = true
							break
						}
					}
				case NOKEYMATCH:
					regex, err := regexp.Compile(conditionValue)
					if err != nil {
						continue
					}
					hasMatch := false
					for key := range data.ParsedData {
						if regex.MatchString(key) {
							hasMatch = true
							break
						}
					}
					if !hasMatch {
						conditionMet = true
					}
				case KEYEQUALS:
					for key := range data.ParsedData {
						if key == conditionValue {
							conditionMet = true
							break
						}
					}
				case KEYDOESNOTEQUAL:
					hasEqual := false
					for key := range data.ParsedData {
						if key == conditionValue {
							hasEqual = true
							break
						}
					}
					if !hasEqual {
						conditionMet = true
					}
				case VALUEEQUALS:
					for _, value := range data.ParsedData {
						if strValue, ok := value.(string); ok && strValue == conditionValue {
							conditionMet = true
							break
						}
					}
				case VALUEDOESNOTEQUAl:
					hasEqual := false
					for _, value := range data.ParsedData {
						if strValue, ok := value.(string); ok && strValue == conditionValue {
							hasEqual = true
							break
						}
					}
					if !hasEqual {
						conditionMet = true
					}
				}
				if conditionMet {
					break
				}
			}
			if !conditionMet {
				return data, nil
			}
		}
	}

	// Create a copy of the parsed data to work with
	modifiedData := make(map[string]any)
	maps.Copy(modifiedData, data.ParsedData)

	// Apply set operations (overwrite existing keys)
	for key, value := range m.set {
		modifiedData[key] = value
	}

	// Apply add operations (only if key doesn't exist)
	for key, value := range m.add {
		if _, exists := modifiedData[key]; !exists {
			modifiedData[key] = value
		}
	}

	// Apply rename operations (if old key exists and new key doesn't)
	for oldKey, newKey := range m.rename {
		if value, exists := modifiedData[oldKey]; exists {
			if _, newKeyExists := modifiedData[newKey]; !newKeyExists {
				modifiedData[newKey] = value
				delete(modifiedData, oldKey)
			}
		}
	}

	// Apply hard rename operations (if old key exists)
	for oldKey, newKey := range m.hardRename {
		if value, exists := modifiedData[oldKey]; exists {
			modifiedData[newKey] = value
			delete(modifiedData, oldKey)
		}
	}

	// Apply remove operations for exact matches
	for _, key := range m.remove {
		delete(modifiedData, key)
	}

	// Apply remove operations for regex matches
	for _, regex := range m.removeRegex {
		for key := range modifiedData {
			if regex.MatchString(key) {
				delete(modifiedData, key)
			}
		}
	}

	// Apply remove operations for wildcard matches
	for _, pattern := range m.removeWildcard {
		for key := range modifiedData {
			if util.GlobMatch(key, pattern) {
				delete(modifiedData, key)
			}
		}
	}

	// Update the event with modified data
	data.ParsedData = modifiedData
	return data, nil
}
