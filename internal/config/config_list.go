package config

import (
	"os"
	"sort"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/uozi-tech/cosy/logger"
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
	case "status":
		flag = c.ConfigList[i].Status > c.ConfigList[j].Status
	case "namespace_id":
		flag = c.ConfigList[i].NamespaceID > c.ConfigList[j].NamespaceID
	}

	if c.ConfigList[i].IsDir != c.ConfigList[j].IsDir {
		// Sort folders and files separately
		flag = boolToInt(c.ConfigList[i].IsDir) < boolToInt(c.ConfigList[j].IsDir)
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

func GetConfigList(relativePath string, filter func(file os.FileInfo) bool) ([]Config, error) {
	configFiles, err := os.ReadDir(nginx.GetConfPath(relativePath))
	if err != nil {
		return nil, err
	}

	configs := make([]Config, 0)

	for i := range configFiles {
		file := configFiles[i]
		fileInfo, err := file.Info()
		if err != nil {
			logger.Error("Get File Info Error", file.Name(), err)
			continue
		}

		if filter != nil && !filter(fileInfo) {
			continue
		}

		switch mode := fileInfo.Mode(); {
		case mode.IsRegular(): // regular file, not a hidden file
			if "." == file.Name()[0:1] {
				continue
			}
		case mode&os.ModeSymlink != 0: // is a symbol
			var targetPath string
			targetPath, err = os.Readlink(nginx.GetConfPath(relativePath, file.Name()))
			if err != nil {
				logger.Error("Read Symlink Error", targetPath, err)
				continue
			}

			var targetInfo os.FileInfo
			targetInfo, err = os.Stat(targetPath)
			if err != nil {
				logger.Error("Stat Error", targetPath, err)
				continue
			}
			// hide the file if it's target file is a directory
			if targetInfo.IsDir() {
				continue
			}
		}

		configs = append(configs, Config{
			Name:       file.Name(),
			ModifiedAt: fileInfo.ModTime(),
			Size:       fileInfo.Size(),
			IsDir:      fileInfo.IsDir(),
		})
	}

	return configs, nil
}
