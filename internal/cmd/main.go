package cmd

import (
	"context"
	"log"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/user"
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
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "pidfile",
						Usage: "`PATH` to the PID file",
						Action: func(ctx context.Context, command *cli.Command, s string) error {
							// remove `pidfile` parameter from os.Args
							for i, arg := range os.Args {
								if arg == "--pidfile" || arg == "-p" {
									os.Args = append(os.Args[:i], os.Args[i+2:]...)
									break
								}
							}
							return nil
						},
					},
				},
			},
			{
				Name:   "reset-password",
				Usage:  "Reset the initial user password",
				Action: user.ResetInitUserPassword,
			},
			UpgradeDockerStep2Command,
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

	// Set the version printer
	cli.VersionPrinter = VersionPrinter

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	} else if !serve {
		os.Exit(0)
	}
	return cmd
}
