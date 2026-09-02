package nginx

import (
	"context"

	"github.com/0xJacky/Nginx-UI/internal/docker"
)

// dockerRunner executes commands inside an external Docker container
// identified by settings.NginxSettings.ContainerName.
type dockerRunner struct{}

func (d *dockerRunner) Exec(ctx context.Context, name string, args ...string) (string, error) {
	cmd := append([]string{name}, args...)
	return docker.Exec(ctx, cmd)
}

func (d *dockerRunner) Stat(path string) bool {
	return docker.StatPath(path)
}

// GOOS is always linux: docker exec targets a Linux container even when
// nginx-ui itself runs on Windows or macOS.
func (d *dockerRunner) GOOS() string {
	return "linux"
}

func newDockerRunner() Runner { return &dockerRunner{} }
