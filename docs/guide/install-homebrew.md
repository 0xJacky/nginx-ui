# Install with Homebrew

This installation method is designed for macOS and Linux users who have already installed Homebrew.

## Prerequisites

- **macOS**: macOS 11 Big Sur or later (amd64 / arm64)
- **Linux**: Most modern Linux distributions (Ubuntu, Debian, CentOS, etc.)
- [Homebrew](https://brew.sh/) installed on your system

If you don't have Homebrew installed, you can install it with:

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

## Installation

### Install Nginx UI

```bash
brew install 0xjacky/tools/nginx-ui
```

This command will:
- Add the `0xjacky/tools` tap to your Homebrew
- Download and install the latest stable version of Nginx UI
- Set up the necessary dependencies
- Create default configuration files and directories

### Verify Installation

After installation, you can verify that Nginx UI is installed correctly:

```bash
nginx-ui --version
```

## Service Management

Nginx UI can be managed as a system service using Homebrew's service management features.

### Start Service

```bash
# Start the service and enable it to start at boot
brew services start nginx-ui

# Or start the service for the current session only
brew services run nginx-ui
```

### Stop Service

```bash
brew services stop nginx-ui
```

### Restart Service

```bash
brew services restart nginx-ui
```

### Check Service Status

```bash
brew services list | grep nginx-ui
```

## Running Manually

If you prefer to run Nginx UI manually instead of as a service:

```bash
# Run in foreground
nginx-ui

# Run with custom config
nginx-ui serve -config /path/to/your/app.ini

# Run in background
nohup nginx-ui serve &
```

## Configuration

The configuration file is automatically created during installation and located at:

- **macOS (Apple Silicon)**: `/opt/homebrew/etc/nginx-ui/app.ini`
- **macOS (Intel)**: `/usr/local/etc/nginx-ui/app.ini`
- **Linux**: `/home/linuxbrew/.linuxbrew/etc/nginx-ui/app.ini`

Data is stored in:
- **macOS (Apple Silicon)**: `/opt/homebrew/var/nginx-ui/`
- **macOS (Intel)**: `/usr/local/var/nginx-ui/`
- **Linux**: `/home/linuxbrew/.linuxbrew/var/nginx-ui/`

The default configuration includes:
```ini
[app]
PageSize = 10

[server]
Host = 0.0.0.0
Port = 9000
RunMode = release

[cert]
HTTPChallengePort = 9180

[terminal]
StartCmd = login
```

## Updating

### Update Nginx UI

```bash
brew upgrade nginx-ui
```

### Update Homebrew and all packages

```bash
brew update && brew upgrade
```

## Uninstallation

### Stop and Uninstall

```bash
# Stop the service first
brew services stop nginx-ui

# Uninstall the package
brew uninstall nginx-ui
```

### Remove the Tap (Optional)

If you no longer need the tap:

```bash
brew untap 0xjacky/tools
```

### Remove Configuration and Data

::: warning

This will permanently delete all your configurations, sites, certificates, and data. Make sure to backup any important data before proceeding.

:::

```bash
# macOS (Apple Silicon)
sudo rm -rf /opt/homebrew/etc/nginx-ui/
sudo rm -rf /opt/homebrew/var/nginx-ui/

# macOS (Intel)
sudo rm -rf /usr/local/etc/nginx-ui/
sudo rm -rf /usr/local/var/nginx-ui/

# Linux
sudo rm -rf /home/linuxbrew/.linuxbrew/etc/nginx-ui/
sudo rm -rf /home/linuxbrew/.linuxbrew/var/nginx-ui/
```

## Troubleshooting

### Port Conflicts

If you encounter port conflicts (default port is 9000), you need to modify the configuration file:

1. **Edit the configuration file:**
   ```bash
   # macOS (Apple Silicon)
   sudo nano /opt/homebrew/etc/nginx-ui/app.ini

   # macOS (Intel)
   sudo nano /usr/local/etc/nginx-ui/app.ini

   # Linux
   sudo nano /home/linuxbrew/.linuxbrew/etc/nginx-ui/app.ini
   ```

2. **Change the port in the `[server]` section:**
   ```ini
   [server]
   Host = 0.0.0.0
   Port = 9001
   RunMode = release
   ```

3. **Restart the service:**
   ```bash
   brew services restart nginx-ui
   ```

### Viewing Service Logs

To troubleshoot service issues, you can view the logs using these commands:

#### Homebrew Service Logs

Nginx UI's Homebrew formula includes proper log configuration:

```bash
# View service status and log file paths
brew services info nginx-ui

# View standard output logs
tail -f $(brew --prefix)/var/log/nginx-ui.log

# View error logs
tail -f $(brew --prefix)/var/log/nginx-ui.err.log

# View both logs simultaneously
tail -f $(brew --prefix)/var/log/nginx-ui.log $(brew --prefix)/var/log/nginx-ui.err.log
```

#### systemd Logs (Linux)

For Linux systems using systemd:

```bash
# View service logs
journalctl -u homebrew.mxcl.nginx-ui -f

# View recent logs
journalctl -u homebrew.mxcl.nginx-ui --since "1 hour ago"
```

#### Manual Debugging

If you need to debug service issues, you can run manually to see output:

```bash
# Run in foreground to see all output
nginx-ui serve -config $(brew --prefix)/etc/nginx-ui/app.ini

# Check if the service is running
ps aux | grep nginx-ui
```

### Permission Issues

If you encounter permission issues when managing Nginx configurations:

1. Make sure your user has the necessary permissions to read/write Nginx configuration files
2. You might need to run Nginx UI with elevated privileges for certain operations
3. Check file permissions:
   ```bash
   # Check configuration file permissions
   ls -la $(brew --prefix)/etc/nginx-ui/app.ini

   # Check data directory permissions
   ls -la $(brew --prefix)/var/nginx-ui/
   ```

### Service Won't Start

If the service fails to start:

1. **Check the service status:**
   ```bash
   brew services list | grep nginx-ui
   ```

2. **Verify the configuration file exists and is valid:**
   ```bash
   # Check if config file exists
   ls -la $(brew --prefix)/etc/nginx-ui/app.ini

   # Test configuration
   nginx-ui serve -config $(brew --prefix)/etc/nginx-ui/app.ini --help
   ```

3. **Try running manually to see error messages:**
   ```bash
   nginx-ui serve -config $(brew --prefix)/etc/nginx-ui/app.ini
   ```

4. **Check for port conflicts:**
   ```bash
   # Check if port 9000 is already in use
   lsof -i :9000

   # Check if HTTP challenge port is in use
   lsof -i :9180
   ```

## Getting Help

If you encounter any issues:

1. Check the [official documentation](https://nginxui.com)
2. Search for existing issues on [GitHub](https://github.com/0xJacky/nginx-ui/issues)
3. Create a new issue if your problem isn't already reported

## Next Steps

After installation, you can:

1. Access the web interface at `http://localhost:9000`
2. Complete the initial setup wizard
3. Start configuring your Nginx sites
4. Explore the [configuration guides](./config-server) for advanced setups
