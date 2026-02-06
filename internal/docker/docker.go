package docker

import (
	"context"

	"github.com/docker/docker/client"
)

// dockerAPIVersion specifies the Docker API version to use for all client operations.
// Version 1.41 is stable and widely supported, ensuring compatibility with archive endpoints
// used by ContainerStatPath and other container operations.
const dockerAPIVersion = "1.41"

// Initialize Docker client from environment variables
// Uses explicit API version to ensure compatibility with archive endpoints
func initClient() (cli *client.Client, err error) {
	// Use explicit API version instead of negotiation to avoid compatibility issues
	// with endpoints like ContainerStatPath that may not be available in all API versions
	cli, err = client.NewClientWithOpts(client.FromEnv, client.WithVersion(dockerAPIVersion))
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
