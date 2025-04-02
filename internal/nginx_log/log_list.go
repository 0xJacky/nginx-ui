package nginx_log

import (
	"slices"

	"github.com/0xJacky/Nginx-UI/internal/cache"
)

func typeToInt(t string) int {
	if t == "access" {
		return 0
	}
	return 1
}

func sortCompare(i, j *cache.NginxLogCache, key string, order string) bool {
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

func Sort(key string, order string, configs []*cache.NginxLogCache) []*cache.NginxLogCache {
	slices.SortStableFunc(configs, func(i, j *cache.NginxLogCache) int {
		if sortCompare(i, j, key, order) {
			return 1
		}
		return -1
	})
	return configs
}
