package settings

type Cluster struct {
	Node []string `ini:",,allowshadow"`
}

var ClusterSettings = Cluster{
	Node: []string{},
}
