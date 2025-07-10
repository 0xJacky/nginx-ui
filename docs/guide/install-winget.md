# Install with Winget

This installation method is designed for Windows users who have the Windows Package Manager (winget) available.

## Prerequisites

- **Windows**: Windows 10 version 1709 (build 16299) or later
- [Windows Package Manager (winget)](https://learn.microsoft.com/en-us/windows/package-manager/winget/) installed

If you don't have winget installed, you can install it from the [Microsoft Store](https://www.microsoft.com/store/productId/9NBLGGH4NNS1) or download it from the [GitHub releases page](https://github.com/microsoft/winget-cli/releases).

## Installation

### Install Nginx UI

```powershell
winget install 0xJacky.nginx-ui
```

This command will:
- Download and install the latest stable version of Nginx UI to `%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\`
- Set up the necessary dependencies
- Add nginx-ui to your system PATH

**Note**: The installation process does not create any configuration files. You will need to create the configuration manually or let Nginx UI create it on first run.

### Verify Installation

After installation, you can verify that Nginx UI is installed correctly:

```powershell
nginx-ui --version
```

### Installation Directory

WinGet installs Nginx UI to the user's local directory:
- **Installation Path**: `%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\`
- **Executable Path**: `%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\nginx-ui.exe`

You can access this directory using:
```powershell
cd "%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\"
```

## Service Management

On Windows, Nginx UI can be run as a Windows Service or manually started from the command line.

### Install as Windows Service

Since Nginx UI doesn't have built-in Windows service management, you need to manually register it using `sc.exe`:

```powershell
# Create the service (run as Administrator)
# Note: WinGet installs to user's local directory
sc create nginx-ui binPath= "%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\nginx-ui.exe serve" start= auto

# Start the service
sc start nginx-ui
```

### Manual Service Management

You can manage the service using Windows Service Manager or PowerShell:

```powershell
# Start the service
Start-Service nginx-ui

# Stop the service
Stop-Service nginx-ui

# Restart the service
Restart-Service nginx-ui

# Check service status
Get-Service nginx-ui
```

### Set Service to Start Automatically

The service is already configured to start automatically with `start= auto` in the creation command above. To change this later:

```powershell
Set-Service -Name nginx-ui -StartupType Automatic
```

## Running Manually

If you prefer to run Nginx UI manually instead of as a service:

```powershell
# Run in foreground
nginx-ui

# Run with custom config
nginx-ui serve -config C:\path\to\your\app.ini

# Run directly from installation directory
"%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\nginx-ui.exe" serve

# Run in background (using Start-Job)
Start-Job -ScriptBlock { nginx-ui serve }
```

## Configuration

The configuration file needs to be created manually or will be created on first run. It should be located at:

- **Default Path**: `%LOCALAPPDATA%\nginx-ui\app.ini`
- **Alternative Path**: `C:\ProgramData\nginx-ui\app.ini`

Data is typically stored in:
- `%LOCALAPPDATA%\nginx-ui\`
- `C:\ProgramData\nginx-ui\`

### Creating Configuration

You can either:
1. **Let Nginx UI create it automatically** - Run nginx-ui for the first time and it will create a default configuration in the current working directory
2. **Create manually** - Create the directories and configuration file yourself

To create the configuration directory and file manually:
```powershell
# Create the configuration directory
New-Item -ItemType Directory -Force -Path "$env:LOCALAPPDATA\nginx-ui"

# Create a basic configuration file
@"
[app]
PageSize = 10

[server]
Host = 0.0.0.0
Port = 9000
RunMode = release

[cert]
HTTPChallengePort = 9180

[terminal]
StartCmd = cmd
"@ | Out-File -FilePath "$env:LOCALAPPDATA\nginx-ui\app.ini" -Encoding utf8
```

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
StartCmd = cmd
```

## Updating

### Update Nginx UI

```powershell
winget upgrade nginx-ui
```

### Update all packages

```powershell
winget upgrade --all
```

## Uninstallation

### Stop and Uninstall Service

```powershell
# Stop the service first
sc stop nginx-ui

# Delete the service
sc delete nginx-ui

# Uninstall the package
winget uninstall nginx-ui
```

### Remove Configuration and Data

::: warning

This will permanently delete all your configurations, sites, certificates, and data. Make sure to backup any important data before proceeding.

:::

```powershell
# Remove configuration and data directories
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\nginx-ui"
Remove-Item -Recurse -Force "$env:PROGRAMDATA\nginx-ui"
```

## Troubleshooting

### Port Conflicts

If you encounter port conflicts (default port is 9000), you need to modify the configuration file:

1. **Edit the configuration file:**
   ```powershell
   notepad "$env:LOCALAPPDATA\nginx-ui\app.ini"
   ```

2. **Change the port in the `[server]` section:**
   ```ini
   [server]
   Host = 0.0.0.0
   Port = 9001
   RunMode = release
   ```

3. **Restart the service:**
   ```powershell
   Restart-Service nginx-ui
   ```

### Windows Firewall

If you have issues accessing Nginx UI from other devices, you may need to configure Windows Firewall:

```powershell
# Allow Nginx UI through Windows Firewall (TCP and UDP)
New-NetFirewallRule -DisplayName "Nginx UI TCP" -Direction Inbound -Protocol TCP -LocalPort 9000 -Action Allow
New-NetFirewallRule -DisplayName "Nginx UI UDP" -Direction Inbound -Protocol UDP -LocalPort 9000 -Action Allow
```

### Viewing Service Logs

To troubleshoot service issues, you can view logs:

#### Windows Event Viewer

1. Open Event Viewer (`eventvwr.msc`)
2. Navigate to Windows Logs > Application
3. Look for events from "nginx-ui" source

#### Service Logs

If Nginx UI is configured to write logs to files:

```powershell
# View log files (if configured)
Get-Content "$env:LOCALAPPDATA\nginx-ui\logs\nginx-ui.log" -Tail 50
```

### Permission Issues

If you encounter permission issues:

1. **Run as Administrator:** Some operations may require administrator privileges
2. **Check folder permissions:** Ensure Nginx UI has read/write access to its configuration and data directories
3. **Antivirus software:** Some antivirus programs may interfere with Nginx UI operation

### Service Won't Start

If the service fails to start:

1. **Check service status:**
   ```powershell
   Get-Service nginx-ui
   ```

2. **Verify configuration file exists (create if needed):**
   ```powershell
   Test-Path "$env:LOCALAPPDATA\nginx-ui\app.ini"
   # If it returns False, create the configuration directory and file first
   ```

3. **Try running manually to see error messages:**
   ```powershell
   nginx-ui serve -config "$env:LOCALAPPDATA\nginx-ui\app.ini"
   # Or run directly from installation directory:
   & "$env:LOCALAPPDATA\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\nginx-ui.exe" serve -config "$env:LOCALAPPDATA\nginx-ui\app.ini"
   ```

4. **Check for port conflicts:**
   ```powershell
   # Check if port 9000 is already in use
   netstat -an | findstr :9000
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
