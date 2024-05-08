package settings

type Cluster struct {
	Node []string `ini:",,allowshadow"`
}

var ClusterSettings = Cluster{
	Node: []string{},
}

func ReloadCluster() (err error) {
	err = load()

	if err != nil {
		return err
	}

	return mapTo("cluster", &ClusterSettings)
}
