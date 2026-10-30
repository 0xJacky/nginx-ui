package cmd

import (
	"context"
	"errors"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/demo"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
	"github.com/urfave/cli/v3"
)

// DemoCommand holds the steps resources/demo/entrypoint.sh runs before the
// server starts. Hidden because it only makes sense inside the demo image.
var DemoCommand = &cli.Command{
	Name:   "demo",
	Usage:  "Prepare a public demo instance",
	Hidden: true,
	Commands: []*cli.Command{
		{
			Name:   "install-log-fixtures",
			Usage:  "Write the synthetic nginx access and error logs before nginx starts",
			Action: installDemoLogFixtures,
		},
	},
}

func installDemoLogFixtures(_ context.Context, command *cli.Command) error {
	config := command.String("config")
	if _, err := os.Stat(config); err != nil {
		return err
	}
	settings.Init(config)
	logger.Init("release")

	// A second guard on top of the refuse-to-overwrite one: an empty access log
	// on a fresh self-hosted install must never gain fabricated traffic.
	if !demo.Enabled() {
		return errors.New("demo mode is not enabled in " + config + "; refusing to write fixture logs")
	}

	return demo.InstallLogFixtures()
}
