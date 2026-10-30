package cmd

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func findDemoTestCommand(t *testing.T, name string) *cli.Command {
	t.Helper()

	for _, command := range DemoCommand.Commands {
		if command.Name == name {
			return command
		}
	}

	t.Fatalf("demo subcommand %q not found", name)
	return nil
}

// nginxStart matches the line that launches nginx in the demo entrypoint.
var nginxStart = regexp.MustCompile(`^nginx\s+-g\b`)

// The log dashboard is only populated if the synthetic access log is written
// while access.log is still empty. Once nginx is up, the platform health check
// writes a real line and the refuse-to-overwrite guard skips the fixture for
// the container's whole life, so the call must come first and must not be
// backgrounded.
func TestDemoEntrypointInstallsLogFixturesBeforeNginx(t *testing.T) {
	sub := findDemoTestCommand(t, "install-log-fixtures")
	invocation := DemoCommand.Name + " " + sub.Name

	script, err := os.ReadFile(filepath.Join("..", "..", "resources", "demo", "entrypoint.sh"))
	require.NoError(t, err)

	fixtureLine, nginxLine := -1, -1
	for i, line := range strings.Split(string(script), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if fixtureLine < 0 && strings.Contains(line, "nginx-ui ") && strings.Contains(line, invocation) {
			fixtureLine = i
			assert.False(t, strings.HasSuffix(line, "&"),
				"the fixture install must finish before nginx starts, not run in the background")
		}
		if nginxLine < 0 && nginxStart.MatchString(line) {
			nginxLine = i
		}
	}

	require.GreaterOrEqual(t, fixtureLine, 0, "entrypoint.sh no longer runs `nginx-ui %s`", invocation)
	require.GreaterOrEqual(t, nginxLine, 0, "entrypoint.sh no longer starts nginx with `nginx -g`")
	assert.Less(t, fixtureLine, nginxLine,
		"entrypoint.sh must install the log fixtures before it starts nginx")
}

func TestDemoInstallLogFixturesRefusesWithoutDemoMode(t *testing.T) {
	config := filepath.Join(t.TempDir(), "app.ini")
	require.NoError(t, os.WriteFile(config, []byte("[node]\nDemo = false\n"), 0o644))

	app := &cli.Command{
		Name:     "nginx-ui",
		Flags:    []cli.Flag{&cli.StringFlag{Name: "config"}},
		Commands: []*cli.Command{DemoCommand},
	}
	err := app.Run(context.Background(),
		[]string{"nginx-ui", "--config", config, "demo", "install-log-fixtures"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "demo mode is not enabled")
}
