package cmd

import (
	"testing"

	"github.com/urfave/cli/v3"
)

func findCertTestCommand(t *testing.T, name string) *cli.Command {
	t.Helper()

	for _, command := range CertCommand.Commands {
		if command.Name == name {
			return command
		}
	}

	t.Fatalf("cert subcommand %q not found", name)
	return nil
}

func certTestFlagNames(flags []cli.Flag) map[string]struct{} {
	names := make(map[string]struct{}, len(flags))
	for _, flag := range flags {
		switch f := flag.(type) {
		case *cli.BoolFlag:
			names[f.Name] = struct{}{}
		case *cli.StringFlag:
			names[f.Name] = struct{}{}
		}
	}
	return names
}

func TestCertScanCommandDoesNotExposeFreeFormDiscoveryFlags(t *testing.T) {
	scan := findCertTestCommand(t, "scan")
	flags := certTestFlagNames(scan.Flags)

	for _, name := range []string{"dir", "pattern", "configured"} {
		if _, ok := flags[name]; ok {
			t.Fatalf("cert scan must not expose free-form discovery flag %q", name)
		}
	}
	for _, name := range []string{"new-only", "name-prefix", "key-type"} {
		if _, ok := flags[name]; !ok {
			t.Fatalf("cert scan is missing expected flag %q", name)
		}
	}
}

func TestCertImportCommandDoesNotExposeDirectoryDiscoveryFlag(t *testing.T) {
	importCmd := findCertTestCommand(t, "import")
	flags := certTestFlagNames(importCmd.Flags)

	if _, ok := flags["dir"]; ok {
		t.Fatalf("cert import must not expose directory discovery flag")
	}
	for _, name := range []string{"name", "cert", "key", "key-type"} {
		if _, ok := flags[name]; !ok {
			t.Fatalf("cert import is missing expected flag %q", name)
		}
	}
}
