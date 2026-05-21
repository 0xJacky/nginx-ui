# Nginx

In this section, we will introduce configuration options in Nginx UI about Nginx control commands, log paths, and other parameters.

::: tip Tip
Starting from Nginx UI v2.0.0-beta.3, we have renamed the `nginx_log` configuration item to `nginx`.
:::

## Logs
Nginx logs are crucial for monitoring, troubleshooting, and maintaining your web server. They provide valuable insights into server performance, user behavior, and potential issues.

### AccessLogPath

- Type: `string`

This option is used to set the path for Nginx access logs in Nginx UI, allowing us to view log content online.

::: tip Tip
In Nginx UI v2, we parse the output of the `nginx -V` command to get the default path for Nginx access logs.

If you need to set a different path, you can use this option.
:::

### ErrorLogPath

- Type: `string`

This option is used to set the path for Nginx error logs in Nginx UI, allowing us to view log content online.

::: tip Tip
In Nginx UI v2, we parse the output of the `nginx -V` command to get the default path for Nginx error logs.

If you need to set a different path, you can use this option.
:::

### LogDirWhiteList

- Type: `[]string`
- Version：`>= v2.0.0-beta.36`
- Example: `/var/log/nginx,/var/log/sites`

This option is used to set the whitelist of directories for the Nginx logs viewer in Nginx UI.

::: warning Warning
For security reasons, you must specify the directories where the logs are stored. 

Only logs within these directories can be viewed online.
:::

## Service Monitoring and Control

In this section, we will introduce configuration options in Nginx UI for monitoring and controlling Nginx services.

### ConfigDir
- Type: `string`

This option is used to set the path for the Nginx configuration folder.

In Nginx UI v2, we parse the output of the `nginx -V` command to get the default path for the Nginx configuration file.

If you need to override the default path, you can use this option.

### PIDPath
- Type: `string`

This option is used to set the path for the Nginx PID file. Nginx UI determines the running status of the Nginx service by checking if this file exists.

In Nginx UI v2, we parse the output of the `nginx -V` command to get the default path for the Nginx PID file.

If you need to override the default path, you can use this option.

### SbinPath
- Type: `string`
- Version: `>= v2.1.10`

This option is used to set the path for the Nginx executable file.

By default, Nginx UI will try to find the Nginx executable file in `$PATH`.

If you need to override the default path, you can use this option.

### TestConfigCmd
- Type: `string`
- Default: `nginx -t`

This option is used to set the command for testing the Nginx configuration.

### ReloadCmd
- Type: `string`
- Default: `nginx -s reload`

This option is used to set the command for reloading the Nginx configuration.

### RestartCmd
- Type: `string`

::: tip Tip
We recommend users who manage Nginx with systemd to set this value to `systemctl restart nginx`.
Otherwise, after restarting Nginx in the Nginx UI, you will not be able to get the accurate status of Nginx in systemctl.
:::

If this option is left empty, Nginx UI will use the following command to stop the Nginx service:

```bash
start-stop-daemon --stop --quiet --oknodo --retry=TERM/30/KILL/5 --pidfile $PID
```

If the `--sbin-path` path cannot be obtained from `nginx -V`, Nginx UI will use the following command to start the Nginx service:

```bash
nginx
```

If the `--sbin-path` path can be obtained, Nginx UI will use the following command to start the Nginx service:

```bash
start-stop-daemon --start --quiet --pidfile $PID --exec $SBIN_PATH
```

### StubStatusPort
- Type: `uint`
- Default: `51820`
- Version: `>= v2.0.0-rc.6`

This option is used to set the port for the Nginx stub status module. The stub status module provides basic status information about Nginx, which is used by Nginx UI to monitor the server's performance.

::: tip Tip
Make sure the port you set is not being used by other services.
:::

## Maintenance Page

### MaintenanceTemplate
- Type: `string`
- Environment Variable: `NGINX_UI_NGINX_MAINTENANCE_TEMPLATE`
- Example: `maintenance.html`

This option is used to select a custom HTML template for the Nginx UI maintenance page. You can set it through the environment variable or in Settings > Nginx.

Only the file name is used. Nginx UI loads the custom template from `/etc/nginx/maintenance/<filename>`, and path components in the configured value are ignored.

If this option is empty, the file cannot be read, or the file is empty, Nginx UI falls back to the built-in maintenance page template.

For Docker deployments, mount a host directory to `/etc/nginx/maintenance` and put your template file there:

```yaml
services:
  nginx-ui:
    image: uozi/nginx-ui:latest
    volumes:
      - ./maintenance:/etc/nginx/maintenance
    environment:
      - NGINX_UI_NGINX_MAINTENANCE_TEMPLATE=maintenance.html
```

## Container Control

In this section, we will introduce configuration options in Nginx UI for controlling Nginx services running in another Docker container.

### ContainerName
- Type: `string`
- Version: `>= v2.0.0-rc.6`

This option is used to specify the name of the Docker container where Nginx is running.

If this option is empty, Nginx UI will control the Nginx service on the local machine or within the current container.

If this option is not empty, Nginx UI will control the Nginx service running in the specified container.

::: tip Tip
If you are using the official Nginx UI container and want to control Nginx in another container, you must map the host's docker.sock to the Nginx UI container.

For example: `-v /var/run/docker.sock:/var/run/docker.sock`
:::

## Host SSH Control

For deployments where Nginx UI runs in a Docker container but Nginx is installed natively on the host machine (e.g. systemd-managed via apt/yum), Nginx UI provides a third control mode that uses SSH for command execution and bind-mounts for file I/O.

### Constraints

::: warning Constraints
- **Same-host only**: the Nginx UI container and the target nginx process must be on the same physical/virtual machine. For multi-host management, see [Manage Multi-Host Nginx with Cluster](manage-multi-host-nginx-with-cluster.md).
- **systemd required** on the host. The mode invokes `systemctl reload|restart <unit>` for control.
- The host nginx user must allow a dedicated unprivileged user (typically `nginxui`) to invoke a narrow set of commands via `sudo -n` without password.
:::

### Quick start

1. From the Web UI, go to **Preferences → Nginx**, select **Host via SSH** mode, and open the setup wizard.
2. Follow the four-step wizard: generate a keypair, paste the generated docker-compose snippet into your stack, apply the sudoers/authorized_keys snippets on the host, and run the verification.
3. Once all checks pass, save the configuration.

Alternatively, use the CLI:

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui
nginx-ui host-setup test
```

### Configuration fields

| Field | Description |
|---|---|
| `host_mode` | Set to `ssh` to enable this mode |
| `host_address` | Remote `host:port` |
| `host_user` | SSH user on the host |
| `host_auth_method` | SSH authentication method. Use key authentication for the current host SSH setup |
| `host_private_key_path` | Private key path inside the container |
| `host_known_hosts_path` | known_hosts allow-list path inside the container |
| `host_sudo_prefix` | Prefix used for privileged commands. Default `sudo -n` |
| `host_systemd_unit_name` | Default `nginx.service` |
| `host_systemctl_path` | Default `/bin/systemctl` |
| `host_config_dir` | Host-side nginx config directory |
| `host_log_dir` | Host-side nginx log directory |

See also: [Manage Host Nginx from Docker](manage-host-nginx-from-docker.md) and [Manage Multi-Host Nginx with Cluster](manage-multi-host-nginx-with-cluster.md).
