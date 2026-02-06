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
	// Ping the server to ensure the connection is valid and negotiate API version
	ping, err := cli.Ping(context.Background())
	if err != nil {
		return
	}
	// Explicitly negotiate API version based on the ping response
	// This ensures the client uses a compatible API version with the Docker daemon
	cli.NegotiateAPIVersionPing(ping)

	return
}
