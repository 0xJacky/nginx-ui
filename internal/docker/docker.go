package docker

import (
	"context"

	"github.com/docker/docker/client"
)

// Initialize Docker client from environment variables
func initClient() (cli *client.Client, err error) {
	cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	// Optionally ping the server to ensure the connection is valid
	_, err = cli.Ping(context.Background())
	if err != nil {
		return
	}

	return
}
