package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/urfave/cli/v3"
)

// PluginCommand manages the installed plugins without going through the HTTP
// API, which is what a recovery shell or an automated deployment needs.
var PluginCommand = &cli.Command{
	Name:  "plugin",
	Usage: "Manage plugins",
	Commands: []*cli.Command{
		{
			Name:   "list",
			Usage:  "List the installed plugins",
			Action: ListPlugins,
		},
		{
			Name:      "install",
			Usage:     "Install or upgrade a plugin from a package",
			ArgsUsage: "<package.tar.gz>",
			Action:    InstallPlugin,
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "enable", Usage: "enable the plugin after installing it"},
				&cli.BoolFlag{Name: "approve-permissions", Usage: "approve the permission set the package asks for"},
			},
		},
		{
			Name:      "uninstall",
			Usage:     "Remove a plugin and its data",
			ArgsUsage: "<plugin-id>",
			Action:    UninstallPlugin,
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "cascade", Usage: "also remove the plugins that require it"},
			},
		},
		{
			Name:      "enable",
			Usage:     "Enable a plugin",
			ArgsUsage: "<plugin-id>",
			Action:    EnablePlugin,
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "approve-permissions", Usage: "approve the permission set the plugin asks for"},
			},
		},
		{
			Name:      "disable",
			Usage:     "Disable a plugin",
			ArgsUsage: "<plugin-id>",
			Action:    DisablePlugin,
		},
		{
			Name:      "inspect",
			Usage:     "Show what a package contains without installing it",
			ArgsUsage: "<package.tar.gz>",
			Action:    InspectPlugin,
		},
		{
			Name:      "pack",
			Usage:     "Build a plugin package from a directory",
			ArgsUsage: "<plugin-dir> <package.tar.gz>",
			Action:    PackPlugin,
		},
	},
}

// ListPlugins prints the plugin inventory of this node.
func ListPlugins(ctx context.Context, command *cli.Command) error {
	manager, err := loadPluginManager(ctx, command)
	if err != nil {
		return err
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "ID\tVERSION\tSTATUS\tENABLED\tCAPABILITIES")
	for _, info := range manager.List() {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%t\t%s\n",
			info.ID, info.Version, info.Status, info.Enabled, strings.Join(info.Capabilities, ","))
	}
	return writer.Flush()
}

// InstallPlugin unpacks a package into the plugin directory.
func InstallPlugin(ctx context.Context, command *cli.Command) error {
	archivePath := command.Args().Get(0)
	if archivePath == "" {
		return plugin.ErrPackageInvalid
	}

	manager, err := loadPluginManager(ctx, command)
	if err != nil {
		return err
	}

	info, err := manager.Install(ctx, archivePath, plugin.InstallOptions{
		Enable:             command.Bool("enable"),
		ApprovePermissions: command.Bool("approve-permissions"),
	})
	if err != nil {
		return err
	}

	fmt.Printf("installed %s %s (%s)\n", info.ID, info.Version, info.Status)
	return nil
}

// UninstallPlugin removes a plugin and everything it stored.
func UninstallPlugin(ctx context.Context, command *cli.Command) error {
	id := command.Args().Get(0)
	if id == "" {
		return plugin.ErrPluginNotFound
	}

	manager, err := loadPluginManager(ctx, command)
	if err != nil {
		return err
	}
	if err = manager.Uninstall(ctx, id, command.Bool("cascade")); err != nil {
		return err
	}

	fmt.Printf("uninstalled %s\n", id)
	return nil
}

// EnablePlugin marks a plugin as wanted. The process itself starts with the
// next server run.
func EnablePlugin(ctx context.Context, command *cli.Command) error {
	id := command.Args().Get(0)
	if id == "" {
		return plugin.ErrPluginNotFound
	}

	manager, err := loadPluginManager(ctx, command)
	if err != nil {
		return err
	}

	info, err := manager.Enable(ctx, id, command.Bool("approve-permissions"))
	if err != nil {
		return err
	}

	fmt.Printf("enabled %s (%s)\n", info.ID, info.Status)
	return nil
}

// DisablePlugin marks a plugin as unwanted.
func DisablePlugin(ctx context.Context, command *cli.Command) error {
	id := command.Args().Get(0)
	if id == "" {
		return plugin.ErrPluginNotFound
	}

	manager, err := loadPluginManager(ctx, command)
	if err != nil {
		return err
	}

	info, err := manager.Disable(ctx, id)
	if err != nil {
		return err
	}

	fmt.Printf("disabled %s (%s)\n", info.ID, info.Status)
	return nil
}

// InspectPlugin prints what a package would install.
func InspectPlugin(ctx context.Context, command *cli.Command) error {
	archivePath := command.Args().Get(0)
	if archivePath == "" {
		return plugin.ErrPackageInvalid
	}

	manager, err := loadPluginManager(ctx, command)
	if err != nil {
		return err
	}

	result, err := manager.Inspect(archivePath)
	if err != nil {
		return err
	}

	manifest := result.Manifest
	fmt.Printf("id: %s\n", manifest.ID)
	fmt.Printf("name: %s\n", manifest.Name)
	fmt.Printf("version: %s\n", manifest.Version)
	fmt.Printf("api version: %d\n", manifest.APIVersion)
	if manifest.MinNginxUIVersion != "" {
		fmt.Printf("min nginx-ui version: %s\n", manifest.MinNginxUIVersion)
	}
	fmt.Printf("capabilities: %s\n", strings.Join(manifest.Capabilities, ", "))
	fmt.Printf("permissions: %s\n", strings.Join(result.Permissions, ", "))
	fmt.Printf("platforms: %s\n", strings.Join(result.Platforms, ", "))
	fmt.Printf("runs on this node (%s): %t\n", result.HostPlatform, result.PlatformSupported)
	if result.InstalledVersion != "" {
		fmt.Printf("installed version: %s\n", result.InstalledVersion)
		fmt.Printf("permissions changed: %t\n", result.PermissionsChanged)
	}
	for _, requirement := range result.RequiresMissing {
		fmt.Printf("missing dependency: %s %s\n", requirement.ID, requirement.Version)
	}
	return nil
}

// PackPlugin builds a distributable package from a plugin directory. It needs
// neither settings nor a database.
func PackPlugin(_ context.Context, command *cli.Command) error {
	sourceDir := command.Args().Get(0)
	archivePath := command.Args().Get(1)
	if sourceDir == "" || archivePath == "" {
		return plugin.ErrPackageInvalid
	}

	manifest, err := plugin.LoadManifest(sourceDir)
	if err != nil {
		return err
	}
	if err = plugin.ValidateManifest(manifest); err != nil {
		return err
	}
	if err = plugin.BuildPackage(sourceDir, archivePath); err != nil {
		return err
	}

	fmt.Printf("packed %s %s into %s\n", manifest.ID, manifest.Version, archivePath)
	return nil
}

// loadPluginManager boots the settings and the database, then reads the plugin
// inventory without starting any plugin process.
func loadPluginManager(ctx context.Context, command *cli.Command) (*plugin.Manager, error) {
	if err := initCertCommand(command); err != nil {
		return nil, err
	}
	manager := plugin.GetManager()
	if err := manager.LoadOffline(ctx); err != nil {
		return nil, err
	}
	return manager, nil
}
