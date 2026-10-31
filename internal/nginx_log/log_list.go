package nginx_log

import (
	"slices"
)

// typeToInt converts log type string to a sortable integer
// "access" = 0, "error" = 1
func typeToInt(t string) int {
	if t == "access" {
		return 0
	}
	return 1
}

// sortCompare compares two log entries based on the specified key and order
// Returns true if i should come after j in the sorted list
func sortCompare(i, j *NginxLogCache, key string, order string) bool {
	flag := false

	switch key {
	case "type":
		flag = typeToInt(i.Type) > typeToInt(j.Type)
	default:
		fallthrough
	case "name":
		flag = i.Name > j.Name
	}

	if order == "asc" {
		flag = !flag
	}

	return flag
}

// Sort sorts a list of NginxLogCache entries by the specified key and order
// Supported keys: "type", "name"
// Supported orders: "asc", "desc"
func Sort(key string, order string, configs []*NginxLogCache) []*NginxLogCache {
	slices.SortStableFunc(configs, func(i, j *NginxLogCache) int {
		if sortCompare(i, j, key, order) {
			return 1
		}
		return -1
	})
	return configs
}
