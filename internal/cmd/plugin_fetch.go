package cmd

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/urfave/cli/v3"
)

// The marketplace subcommands live in their own file so the plugin command
// itself stays untouched.
func init() {
	PluginCommand.Commands = append(PluginCommand.Commands,
		&cli.Command{
			Name:      "fetch",
			Usage:     "Download a plugin package and its signature for an offline install",
			ArgsUsage: "<plugin-id>",
			Action:    FetchPlugin,
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "version", Usage: "download this version instead of the newest one"},
				&cli.StringFlag{Name: "out", Usage: "destination directory", Value: "."},
				&cli.StringFlag{
					Name:  "platform",
					Usage: `"<goos>-<goarch>" to download the package for, a comma separated list, or "all"; defaults to this node`,
				},
			},
		},
		&cli.Command{
			Name:  "marketplace",
			Usage: "Browse the plugin marketplace",
			Commands: []*cli.Command{
				{
					Name:   "list",
					Usage:  "List the plugins the configured catalogs offer",
					Action: ListMarketplacePlugins,
					Flags: []cli.Flag{
						&cli.StringFlag{Name: "keyword", Usage: "filter by id, name, description or author"},
						&cli.StringFlag{Name: "category", Usage: "filter by category"},
						&cli.BoolFlag{Name: "refresh", Usage: "bypass the catalog cache"},
					},
				},
			},
		},
	)
}

// FetchPlugin downloads one package so it can be carried to an offline node.
func FetchPlugin(ctx context.Context, command *cli.Command) error {
	id := command.Args().Get(0)
	if id == "" {
		return plugin.ErrPluginNotFound
	}

	manager, err := loadPluginManager(ctx, command)
	if err != nil {
		return err
	}

	destination := command.String("out")
	if destination == "" {
		destination = "."
	}

	marketplace := manager.Marketplace()
	wantVersion := command.String("version")
	platforms := splitPlatforms(command.String("platform"))
	if len(platforms) == 1 && platforms[0] == "all" {
		// Every platform comes from the same release, even when a newer one
		// exists for some of them only.
		wantVersion, platforms, err = marketplace.FetchTargets(ctx, id, wantVersion)
		if err != nil {
			return err
		}
	}

	for _, platform := range platforms {
		archive, err := marketplace.FetchPackage(ctx, id, wantVersion, platform, destination)
		if err != nil {
			if platform == "" {
				return err
			}
			return fmt.Errorf("%s: %w", platform, err)
		}
		fmt.Printf("downloaded %s\n", archive)
	}
	return nil
}

// splitPlatforms reads the --platform flag. An empty flag is one empty entry,
// which FetchPackage takes as the platform of this node.
func splitPlatforms(value string) []string {
	platforms := make([]string, 0, 4)
	for _, item := range strings.Split(value, ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" || slices.Contains(platforms, item) {
			continue
		}
		platforms = append(platforms, item)
	}
	if len(platforms) == 0 {
		return []string{""}
	}
	if slices.Contains(platforms, "all") {
		return []string{"all"}
	}
	return platforms
}

// ListMarketplacePlugins prints the merged catalog.
func ListMarketplacePlugins(ctx context.Context, command *cli.Command) error {
	manager, err := loadPluginManager(ctx, command)
	if err != nil {
		return err
	}

	entries, err := manager.Marketplace().Search(ctx, plugin.CatalogFilter{
		Keyword:  command.String("keyword"),
		Category: command.String("category"),
		Refresh:  command.Bool("refresh"),
	})
	if err != nil {
		return err
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "ID\tNAME\tLATEST\tINSTALLED\tTRUST\tCATEGORIES")
	for _, entry := range entries {
		latest := "-"
		if entry.InstallableRelease != nil {
			latest = entry.InstallableRelease.Version
		}
		installed := entry.InstalledVersion
		if installed == "" {
			installed = "-"
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n",
			entry.ID, catalogEntryName(entry), latest, installed, entry.Trust, strings.Join(entry.Categories, ","))
	}
	return writer.Flush()
}

// catalogEntryName prefers the English name and falls back to the id.
func catalogEntryName(entry plugin.CatalogEntry) string {
	if name, ok := entry.Name["en"]; ok && name != "" {
		return name
	}
	for _, name := range entry.Name {
		if name != "" {
			return name
		}
	}
	return entry.ID
}
