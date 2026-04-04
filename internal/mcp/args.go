package mcp

// GetString safely extracts a string value from the arguments map.
// Returns an empty string if the key doesn't exist or the value is nil.
func GetString(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetBool safely extracts a boolean value from the arguments map.
// Returns false if the key doesn't exist or the value is nil.
func GetBool(args map[string]interface{}, key string) bool {
	if v, ok := args[key]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// GetSlice safely extracts a slice of interface{} from the arguments map.
// Returns nil if the key doesn't exist or the value is nil.
func GetSlice(args map[string]interface{}, key string) []interface{} {
	if v, ok := args[key]; ok && v != nil {
		if s, ok := v.([]interface{}); ok {
			return s
		}
	}
	return nil
}
