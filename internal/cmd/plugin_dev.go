package cmd

// Developer tooling subcommands for the plugin system: lint, conformance and
// init. Unlike the rest of PluginCommand (plugin.go) these need neither the
// database nor a running host, so they are registered here from init()
// instead of editing the shared command list directly.

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/scaffold"
	"github.com/urfave/cli/v3"
)

func init() {
	PluginCommand.Commands = append(PluginCommand.Commands,
		&cli.Command{
			Name:      "lint",
			Usage:     "Check a plugin directory or package against the plugin spec",
			ArgsUsage: "<path>",
			Action:    LintPlugin,
		},
		&cli.Command{
			Name:      "conformance",
			Usage:     "Run a plugin binary through the conformance test cases",
			ArgsUsage: "<path>",
			Action:    ConformancePlugin,
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "capability", Usage: "limit the capability specific cases to this one: dns01, notify, probe, mcp, storage, cert.deploy, security.blocklist, upstream.discovery or log.sink"},
				&cli.DurationFlag{Name: "timeout", Value: 90 * time.Second, Usage: "overall time budget for the run"},
				&cli.StringFlag{Name: "transport", Usage: "run the cases over stdio, grpc or both (default: both when the plugin advertises grpc, stdio otherwise)"},
			},
		},
		&cli.Command{
			Name:      "init",
			Usage:     "Scaffold a new plugin repository",
			ArgsUsage: "<dir>",
			Action:    InitPlugin,
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "plugin id, e.g. io.github.example.mydns", Required: true},
				&cli.StringFlag{Name: "name", Usage: "human readable display name", Required: true},
				&cli.StringFlag{Name: "lang", Usage: "go, python or node", Required: true},
				&cli.StringFlag{Name: "capability", Value: "dns01", Usage: "capability to scaffold"},
			},
		},
	)
}

// LintPlugin runs plugin.Lint and prints every finding as a table, in
// addition to returning an error when the report contains one, which makes
// the process exit non-zero the same way every other cli.Command error does.
func LintPlugin(_ context.Context, command *cli.Command) error {
	path := command.Args().Get(0)
	if path == "" {
		return fmt.Errorf("usage: nginx-ui plugin lint <path>")
	}

	report, err := plugin.Lint(path)
	if err != nil {
		return err
	}

	if len(report.Findings) == 0 {
		fmt.Println("no findings")
		return nil
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "LEVEL\tRULE\tMESSAGE")
	for _, finding := range report.Findings {
		fmt.Fprintf(writer, "%s\t%s\t%s\n", finding.Level, finding.Rule, finding.Message)
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	if report.HasErrors() {
		return fmt.Errorf("plugin lint found errors")
	}
	return nil
}

// ConformancePlugin runs plugin.Conformance and prints every case as a
// table, returning an error when a case failed.
func ConformancePlugin(ctx context.Context, command *cli.Command) error {
	path := command.Args().Get(0)
	if path == "" {
		return fmt.Errorf("usage: nginx-ui plugin conformance <path>")
	}

	opts := plugin.ConformanceOptions{
		Timeout:   command.Duration("timeout"),
		Transport: command.String("transport"),
	}
	if capability := command.String("capability"); capability != "" {
		opts.Capabilities = []string{capability}
	}

	report, err := plugin.Conformance(ctx, path, opts)
	if err != nil {
		return err
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "STATUS\tRULE\tTRANSPORT\tCASE\tDURATION\tMESSAGE")
	for _, c := range report.Cases {
		transport := c.Transport
		if transport == "" {
			transport = "-"
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n", c.Status, c.Rule, transport, c.Name, c.Duration.Round(time.Millisecond), c.Message)
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	if !report.Passed() {
		return fmt.Errorf("plugin conformance found failing cases")
	}
	return nil
}

// InitPlugin scaffolds a new plugin repository at <dir>.
func InitPlugin(_ context.Context, command *cli.Command) error {
	dir := command.Args().Get(0)
	if dir == "" {
		return fmt.Errorf("usage: nginx-ui plugin init <dir> --id <id> --name <name> --lang go|python|node")
	}

	id := command.String("id")
	if err := scaffold.Init(dir, scaffold.InitOptions{
		ID:         id,
		Name:       command.String("name"),
		Lang:       command.String("lang"),
		Capability: command.String("capability"),
	}); err != nil {
		return err
	}

	fmt.Printf("scaffolded %s in %s\n", id, dir)
	return nil
}
