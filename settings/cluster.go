package settings

type Cluster struct {
	Node []string `ini:",,allowshadow"`
}
