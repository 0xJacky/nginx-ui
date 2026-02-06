package docker

import (
	"context"

	"github.com/docker/docker/client"
)

// Initialize Docker client from environment variables
// Uses API version 1.41 explicitly to ensure compatibility with archive endpoints
// API version 1.41 is stable and widely supported across Docker versions
func initClient() (cli *client.Client, err error) {
	// Use explicit API version instead of negotiation to avoid compatibility issues
	// with endpoints like ContainerStatPath that may not be available in all API versions
	cli, err = client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.41"))
	if err != nil {
		return
	}
	// Ping the server to ensure the connection is valid
	_, err = cli.Ping(context.Background())
	if err != nil {
		return
	}

	return
}
