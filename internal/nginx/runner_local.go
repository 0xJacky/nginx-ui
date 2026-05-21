package nginx

import (
	"context"
	"os"
	"os/exec"
)

// localRunner executes commands as child processes of nginx-ui itself.
type localRunner struct{}

func (l *localRunner) Exec(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = GetNginxExeDir()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (l *localRunner) Stat(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
