package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/urfave/cli/v3"
)

func NewAppCmd() *cli.Command {
	serve := false

	cmd := &cli.Command{
		Name:  "nginx-ui",
		Usage: "Yet another Nginx Web UI",
		Commands: []*cli.Command{
			{
				Name:  "serve",
				Usage: "Start the Nginx-UI server",
				Action: func(ctx context.Context, command *cli.Command) error {
					serve = true
					return nil
				},
			},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: "app.ini",
				Usage: "configuration file path",
			},
		},
		DefaultCommand: "serve",
		Version:        version.Version,
	}

	cli.VersionPrinter = func(cmd *cli.Command) {
		fmt.Printf("%s (%d)\n", cmd.Root().Version, version.BuildId)
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	} else if !serve {
		os.Exit(0)
	}
	return cmd
}
