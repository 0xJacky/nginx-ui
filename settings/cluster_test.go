package settings

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCluster(t *testing.T) {
	Init("../app.example.ini")

	assert.Equal(t, []string{
		"http://10.0.0.1:9000?name=node1&node_secret=my-node-secret&enabled=true",
		"http://10.0.0.2:9000?name=node2&node_secret=my-node-secret&enabled=true",
	}, ClusterSettings.Node)
}
