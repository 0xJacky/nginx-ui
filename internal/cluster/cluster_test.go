package cluster

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
)

func Test_parseNodeUrl(t *testing.T) {
	settings.Init("../../app.example.ini")
	t.Log(settings.ClusterSettings.Node)
	node := settings.ClusterSettings.Node[0]

	env, err := parseNodeUrl(node)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "node1", env.Name)
	assert.Equal(t, "http://10.0.0.1:9000", env.URL)
	assert.Equal(t, "my-node-secret", env.Token)
	assert.Equal(t, true, env.Enabled)

	node = settings.ClusterSettings.Node[1]

	env, err = parseNodeUrl(node)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "node2", env.Name)
	assert.Equal(t, "http://10.0.0.2:9000", env.URL)
	assert.Equal(t, "my-node-secret", env.Token)
	assert.Equal(t, true, env.Enabled)

	node = settings.ClusterSettings.Node[2]

	env, err = parseNodeUrl(node)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "node3", env.Name)
	assert.Equal(t, "http://10.0.0.3", env.URL)
	assert.Equal(t, "my-node-secret", env.Token)
	assert.Equal(t, true, env.Enabled)
}
