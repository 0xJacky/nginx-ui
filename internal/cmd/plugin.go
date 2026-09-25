package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"aead.dev/minisign"
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
			Usage:     "Build a plugin package from a directory, signed when a key is given",
			ArgsUsage: "<plugin-dir> <package.tar.gz>",
			Action:    PackPlugin,
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "key", Usage: "minisign secret key file to sign the package with"},
			},
		},
		{
			Name:      "sign",
			Usage:     "Sign an existing plugin package in place",
			ArgsUsage: "<package.tar.gz>",
			Action:    SignPlugin,
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "key", Usage: "minisign secret key file to sign the package with", Required: true},
			},
		},
	},
}

// signPasswordEnv holds the password of the minisign secret key, empty for a
// key without one.
const signPasswordEnv = "NGINX_UI_PLUGIN_SIGN_PASSWORD"

// ListPlugins prints the plugin inventory of this node.
func ListPlugins(ctx context.Context, command *cli.Command) error {
	manager, err := loadPluginManager(ctx, command)
	if err != nil {
		return err
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "ID\tVERSION\tSTATUS\tENABLED\tTRUST\tCAPABILITIES")
	for _, info := range manager.List() {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%t\t%s\t%s\n",
			info.ID, info.Version, info.Status, info.Enabled, info.Trust, strings.Join(info.Capabilities, ","))
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
	fmt.Printf("trust: %s\n", result.Trust)
	if result.Signer != "" {
		fmt.Printf("signer: %s\n", result.Signer)
	}
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

// PackPlugin builds a distributable package from a plugin directory, signed
// when --key is given. It needs neither settings nor a database.
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

	keyPath := command.String("key")
	if keyPath == "" {
		if err = plugin.BuildPackage(sourceDir, archivePath); err != nil {
			return err
		}
		fmt.Printf("packed %s %s into %s, unsigned\n", manifest.ID, manifest.Version, archivePath)
		return nil
	}

	key, err := loadSigningKey(keyPath)
	if err != nil {
		return err
	}
	if err = plugin.BuildSignedPackage(sourceDir, archivePath, key); err != nil {
		return err
	}
	fmt.Printf("packed %s %s into %s, signed with key %016X\n", manifest.ID, manifest.Version, archivePath, key.ID())
	return nil
}

// SignPlugin signs an existing package in place, replacing any previous
// signature. It needs neither settings nor a database.
func SignPlugin(_ context.Context, command *cli.Command) error {
	archivePath := command.Args().Get(0)
	if archivePath == "" {
		return plugin.ErrPackageInvalid
	}

	key, err := loadSigningKey(command.String("key"))
	if err != nil {
		return err
	}
	if err = plugin.SignPackage(archivePath, key); err != nil {
		return err
	}
	fmt.Printf("signed %s with key %016X\n", archivePath, key.ID())
	return nil
}

// loadSigningKey reads a minisign secret key file. An encrypted key is
// decrypted with the password from NGINX_UI_PLUGIN_SIGN_PASSWORD.
func loadSigningKey(path string) (minisign.PrivateKey, error) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return minisign.PrivateKey{}, err
	}
	if minisign.IsEncrypted(encoded) {
		key, err := minisign.PrivateKeyFromFile(os.Getenv(signPasswordEnv), path)
		if err != nil {
			return minisign.PrivateKey{}, fmt.Errorf("decrypt %s: %w", path, err)
		}
		return key, nil
	}
	var key minisign.PrivateKey
	if err = key.UnmarshalText(encoded); err != nil {
		return minisign.PrivateKey{}, fmt.Errorf("read %s: %w", path, err)
	}
	return key, nil
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
