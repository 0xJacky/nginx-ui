package settings

import "github.com/uozi-tech/cosy/settings"

type Cluster struct {
	Node []string `ini:",,allowshadow"`
}

var ClusterSettings = &Cluster{
	Node: []string{},
}

func ReloadCluster() (err error) {
	err = settings.Reload()

	if err != nil {
		return err
	}

	return settings.MapTo("cluster", &ClusterSettings)
}
