# Manage Host Nginx from Docker

Use this guide when Nginx UI runs in Docker and needs to manage an nginx instance installed directly on the same host.

::: info Prerequisites
- Linux host with nginx running under systemd, or macOS with nginx running through `brew services`
- Docker installed on the same host
- Linux: an unprivileged user dedicated to Nginx UI (we use `nginxui` in examples)
- macOS: the login user that owns the Homebrew nginx service
- Two-factor authentication enabled for the Nginx UI administrator: the setup wizard requires a verified two-factor session and otherwise shows **Two-factor authentication required**
:::

For macOS, select **macOS (Homebrew)** in the wizard's **Detect Platform** step. Apple Silicon defaults to `/opt/homebrew`. In that step, the wizard queries Homebrew and parses `nginx -V` over SSH to detect the installed nginx version and the actual executable, configuration, log, PID, and document-root paths. This also handles Intel Homebrew under `/usr/local`. Confirm the service is loaded before continuing:

```bash
brew services info nginx
```

## Step 1: Create the unprivileged user (Linux only)

```bash
sudo useradd -r -s /bin/bash -m -G adm nginxui
```

`-G adm` grants the user read access to /var/log files including nginx logs.

On macOS, use the existing login user that runs `brew services`; do not create a separate service user.

## Step 2: SSH Target — generate the keypair via Nginx UI

Open **Preferences → Nginx → Nginx Control Mode → Edit**, select **Host via SSH**, then click **Open SSH setup wizard**. Editing the control mode requires a verified two-factor session.

The wizard has five steps: **SSH Target**, **Trust & Test**, **Detect Platform**, **Access & Install** and **Verify**. The steps below follow the same order.

In **SSH Target**, choose a private key source:

| Source | Behaviour |
| --- | --- |
| **Generate** | Nginx UI creates an ed25519 keypair at the managed path `/etc/nginx-ui/host_key`. |
| **Existing path** | Nginx UI reads a key that already exists inside the container, at a path you enter. |
| **Paste or upload** | You paste the key or pick a file. Nginx UI stores it at the managed path with mode 0600. |

The chosen source is persisted as the `host_key_source` setting. Encrypted private keys are not supported.

Click **Generate keypair** for the generate flow.

Copy the public key shown. It looks like:

```
ssh-ed25519 AAAAC3...generated nginx-ui@generated
```

Append it to the host user's authorized_keys:

```bash
sudo mkdir -p /home/nginxui/.ssh
echo 'ssh-ed25519 AAAA...' | sudo tee -a /home/nginxui/.ssh/authorized_keys
sudo chown -R nginxui:nginxui /home/nginxui/.ssh
sudo chmod 700 /home/nginxui/.ssh
sudo chmod 600 /home/nginxui/.ssh/authorized_keys
```

::: warning Host key verification
Host SSH mode requires a `known_hosts` allow-list. When the wizard shows a new fingerprint, verify it from the host or another trusted channel before trusting it.
:::

## Step 3: Trust & Test — trust the host identity

Open the **Trust & Test** step and click **Scan host keys**. The wizard compares the SSH host keys presented by the host with the configured `known_hosts` file.

::: warning Verify before trusting
Only trust a key after comparing its fingerprint with a source you already trust. This check protects the SSH connection from accepting the wrong host during setup or key rotation.
:::

Good sources include:

- The host console or provider control panel
- A previous inventory record for the server
- A direct command on the host, such as:

::: code-group

```bash
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

```bash [Manual scan]
ssh-keyscan -p 22 host.docker.internal
```

:::

::: details Manual scan fallback
If automatic scanning is not available, run the `ssh-keyscan` command shown in the wizard from a trusted terminal. Paste the output into **Paste ssh-keyscan output**, compare the fingerprint, then trust the key.
:::

::: tip Host key status
- **unknown_host**: no key is trusted for this host yet.
- **new_algorithm**: this host already has a trusted key, but the scan found another algorithm.
- **changed**: a trusted key for the same algorithm no longer matches. Treat this as a security-sensitive event.
- **trusted**: the scanned key matches `known_hosts`.
:::

After every presented key is trusted, click **Test SSH connection**. The wizard only continues once the connection succeeds.

## Step 4: Detect Platform

In **Detect Platform**, pick the service manager (**Linux (systemd)** or **macOS (Homebrew)**). The wizard runs `nginx -V` over SSH and fills in the nginx executable, configuration, log and PID paths.

Each path field is tagged **Auto-detected** when it matches what the host reported, or **Manual override** when you changed it. An overridden field shows the detected value and a **Restore detected value** action. After changing the nginx executable path, use **Re-detect paths from this executable** to run `nginx -V` again and refresh the config, log and PID paths.

The nginx executable path becomes `sbin_path`. If you leave it empty, Nginx UI falls back to the service manager default (`/usr/sbin/nginx` for systemd, `/opt/homebrew/opt/nginx/bin/nginx` for launchd) and stores that default when the configuration is saved. The sudoers allow-list generated in the next step matches this resolved path exactly.

## Step 5: Access & Install — choose the file access mode

The **Access & Install** step starts with **File access mode**:

| Mode | Behaviour |
| --- | --- |
| **Compatibility (SFTP)** | Nginx UI reads and writes the host nginx configuration and logs entirely over SSH and SFTP. No host directories are mounted into the container. |
| **High performance (mounted)** | Nginx UI reads configuration and logs from bind-mounted host directories. This is faster, but the container must be recreated with the generated mounts. |

The choice is persisted as the `host_access_mode` setting.

::: details What SFTP mode covers
- Configuration files, log files and the certificates written by certificate issuance or renewal are read from and written to the host over SFTP.
- The container cannot watch host files for changes over SFTP, so the configuration index and the certificate discovery are rescanned every 30 seconds instead. A change made on the host outside Nginx UI can take up to 30 seconds to appear in the UI.
- The **shared config directory** self-check is skipped, because no directory is shared with the container.
- Known limitation: the scan for importable certificates in the certificate directory still reads the container filesystem, so it does not find certificates that only exist on the host. Issue or renew certificates from Nginx UI, or use the mounted mode, if you rely on that scan.
:::

## Step 6: Access & Install — install the sudoers entry (Linux only)

The **1. On the nginx host** tab shows a sudoers snippet. Copy it and install via:

```bash
sudo visudo -f /etc/sudoers.d/nginx-ui
```

Paste the snippet, save, exit. visudo will reject the file if the syntax is bad.

Homebrew launchd services run in the login user's domain and do not require a sudoers entry.

## Step 7: Access & Install — apply file permissions

::: details Optional ACL commands
If your nginxui user is non-root, grant it write access to /etc/nginx:

```bash
sudo setfacl -R  -m u:nginxui:rwx /etc/nginx
sudo setfacl -dR -m u:nginxui:rwx /etc/nginx
```
:::

On macOS, the wizard emits read/write checks for the Homebrew config and log paths instead of Linux ACL commands.

## Step 8: Access & Install — update the container

The **2. On the Nginx UI container** tab shows a compose snippet, a full override file and a docker run command. It appears when the container needs changes: the mounted access mode, or a Linux Docker Engine that needs the `host.docker.internal` host-gateway mapping. Merge the snippet into your existing `docker-compose.yml`.

The generated snippet sets `NGINX_UI_DISABLE_BUNDLED_NGINX=true` so the container does not start its bundled nginx service while it controls the host nginx service.

In the mounted access mode, the snippet also bind-mounts the configured config, log and PID directories. Recreate the container after changing between Linux and macOS presets so the new paths are applied. In SFTP mode, no directory mounts are added.

If verification detects paths that differ from the initial preset, return to the container tab, regenerate the compose snippet, and recreate the container before running verification again.

::: tip Persist Nginx UI data
Persist `/etc/nginx-ui` with a Docker volume or bind mount. The host key allow-list is stored at `/etc/nginx-ui/known_hosts` by default, and it should survive image upgrades and container rebuilds.
:::

```bash
docker compose up -d --force-recreate nginx-ui
```

After the host and container changes are applied, run **Setup checks** at the bottom of the step. It runs the platform and privilege checks listed in the next step.

## Step 9: Verify the setup

Open **Verify** and click **Run verification**. The main checks should pass:

::: tip Expected verification result

The **Verify** step runs the nginx checks:

- ✓ ssh_connect: echo ok over ssh
- ✓ nginx_test: configuration file ok

The platform and privilege checks belong to **Setup checks** in the **Access & Install** step, which you already ran after applying the snippets:

- ✓ host_platform: Linux host matches systemd
- ✓ systemctl_is_active: active
- ✓ unit_has_execreload: ExecReload is declared
- ✓ config_dir_writable: /etc/nginx accessible
- ✓ log_dir_readable: /var/log/nginx/access.log readable
- ✓ pid_file_present: /var/run/nginx.pid present
- ✓ sudo_available: sudo -n true succeeded
- ✓ sudoers_coverage: all required entries present

`same_host` and `known_hosts_persistence` are part of the connection group, which only `nginx-ui host-setup test` runs from the CLI.

:::

For macOS, `host_platform` reports Darwin, `launchctl_service_loaded` replaces the two systemd checks, and the sudo checks report that sudo is not required. The config, log, and PID checks use `/opt/homebrew` by default.

If `known_hosts_persistence` is shown as a warning, review your Docker volume or bind mount. The warning does not block saving, but trusted host keys may be lost after a container rebuild if `/etc/nginx-ui` is not persisted.

Click **Save configuration** after the checks pass.

::: tip Custom control commands
If `TestConfigCmd`, `ReloadCmd` or `RestartCmd` are set in the Nginx settings, Nginx UI runs them on the host through `/bin/sh -c` as the SSH user instead of the built-in systemd or launchd commands. Keep them empty unless the generated sudoers entry covers whatever they invoke.
:::

## Troubleshooting

::: details `sudo_available` fails with "sudo: a password is required"
- Check your sudoers file has `NOPASSWD:` not just `(root)`.
- Check the file has correct line continuations (`\` at line endings).
:::

::: details `ssh_connect` fails with "permission denied (publickey)"
- Verify authorized_keys has the right line, owner, and permissions.
- Check sshd_config allows `PubkeyAuthentication yes`.
:::

::: details `ssh_connect` fails after the host SSH key changed
A changed host key can be legitimate, for example after rebuilding the host or rotating SSH keys. It can also indicate a wrong target or a man-in-the-middle attack. Replace the trusted key only after confirming the new fingerprint.

1. Open the **Trust & Test** step.
2. Scan the host keys again.
3. Compare the old and new fingerprints shown by the wizard.
4. Verify the new fingerprint on the host or through your provider control panel.
5. Select the confirmation checkbox and click **Replace trusted key**.

Use **Advanced cleanup** only for `known_hosts` entries that you have verified are no longer used.
:::

::: warning `same_host` warns "remote host detected"
Your `host_address` resolves to a different machine. SSH mode does **not** work cross-host; see [Manage Multi-Host Nginx with Cluster](manage-multi-host-nginx-with-cluster.md).
:::

## CLI reference

Generate a keypair for host SSH:

```bash
nginx-ui host-setup keygen --out /etc/nginx-ui/host_key
```

Print all setup snippets (`--access-mode` is required):

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --access-mode sftp
```

Print only Docker or host-side snippets:

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --access-mode sftp --compose
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --access-mode sftp --host
```

Use `--json`, `--override`, or `--docker-run` when you need machine-readable output, a full compose override, or a docker run command.

Run verification against the current settings:

```bash
nginx-ui host-setup test
```

Unlike the wizard, `test` runs every check group: connection, platform, privileges and nginx.

## Related docs

- [Nginx configuration reference](config-nginx.md#host-ssh-control)
- [Manage Multi-Host Nginx with Cluster](manage-multi-host-nginx-with-cluster.md)
