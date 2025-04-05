package config

import (
	"sort"
)

type ConfigsSort struct {
	Key        string
	Order      string
	ConfigList []Config
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (c ConfigsSort) Len() int {
	return len(c.ConfigList)
}

func (c ConfigsSort) Less(i, j int) bool {
	flag := false

	switch c.Key {
	case "name":
		flag = c.ConfigList[i].Name > c.ConfigList[j].Name
	case "modified_at":
		flag = c.ConfigList[i].ModifiedAt.After(c.ConfigList[j].ModifiedAt)
	case "is_dir":
		flag = boolToInt(c.ConfigList[i].IsDir) > boolToInt(c.ConfigList[j].IsDir)
	case "enabled":
		flag = boolToInt(c.ConfigList[i].Enabled) > boolToInt(c.ConfigList[j].Enabled)
	case "env_group_id":
		flag = c.ConfigList[i].EnvGroupID > c.ConfigList[j].EnvGroupID
	}

	if c.Order == "asc" {
		flag = !flag
	}

	return flag
}

func (c ConfigsSort) Swap(i, j int) {
	c.ConfigList[i], c.ConfigList[j] = c.ConfigList[j], c.ConfigList[i]
}

func Sort(key string, order string, configs []Config) []Config {
	configsSort := ConfigsSort{
		Key:        key,
		ConfigList: configs,
		Order:      order,
	}

	sort.Sort(configsSort)

	return configsSort.ConfigList
}
