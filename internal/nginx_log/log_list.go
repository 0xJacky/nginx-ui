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

// sortCompareWithIndex compares two log entries with index status based on the specified key and order
// Returns true if i should come after j in the sorted list
func sortCompareWithIndex(i, j *NginxLogWithIndex, key string, order string) bool {
	flag := false

	switch key {
	case "type":
		flag = typeToInt(i.Type) > typeToInt(j.Type)
	case "index_status":
		// Sort order: indexed > indexing > not_indexed
		statusOrder := map[string]int{
			IndexStatusIndexed:    3,
			IndexStatusIndexing:   2,
			IndexStatusNotIndexed: 1,
		}
		iOrder := statusOrder[i.IndexStatus]
		jOrder := statusOrder[j.IndexStatus]
		flag = iOrder < jOrder
	case "last_indexed":
		// Sort by last indexed time (more recent first)
		if i.LastIndexed != nil && j.LastIndexed != nil {
			flag = i.LastIndexed.After(*j.LastIndexed)
		} else if i.LastIndexed == nil && j.LastIndexed != nil {
			flag = true // nil comes after non-nil
		} else if i.LastIndexed != nil && j.LastIndexed == nil {
			flag = false // non-nil comes before nil
		}
	case "last_size":
		// Sort by file size
		if i.LastSize != 0 && j.LastSize != 0 {
			flag = i.LastSize > j.LastSize
		} else if i.LastSize == 0 && j.LastSize != 0 {
			flag = true
		} else if i.LastSize != 0 && j.LastSize == 0 {
			flag = false
		}
	case "document_count":
		// Sort by document count
		if i.DocumentCount != 0 && j.DocumentCount != 0 {
			flag = i.DocumentCount > j.DocumentCount
		} else if i.DocumentCount == 0 && j.DocumentCount != 0 {
			flag = true
		} else if i.DocumentCount != 0 && j.DocumentCount == 0 {
			flag = false
		}
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

// SortWithIndex sorts a list of NginxLogWithIndex entries by the specified key and order
// Supported keys: "type", "name", "is_indexed", "last_indexed", "last_size", "document_count"
// Supported orders: "asc", "desc"
func SortWithIndex(key string, order string, configs []*NginxLogWithIndex) []*NginxLogWithIndex {
	slices.SortStableFunc(configs, func(i, j *NginxLogWithIndex) int {
		if sortCompareWithIndex(i, j, key, order) {
			return 1
		}
		return -1
	})
	return configs
}
