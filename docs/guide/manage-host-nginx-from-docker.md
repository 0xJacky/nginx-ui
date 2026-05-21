# Manage Host Nginx from Docker

Use this guide when Nginx UI runs in Docker and needs to manage an nginx instance installed directly on the same host.

::: info Prerequisites
- Linux host with nginx installed and running under systemd
- Docker installed on the same host
- An unprivileged user dedicated to Nginx UI (we use `nginxui` in examples)
:::

## Step 1: Create the unprivileged user

```bash
sudo useradd -r -s /bin/bash -m -G adm nginxui
```

`-G adm` grants the user read access to /var/log files including nginx logs.

## Step 2: Generate the keypair via Nginx UI

Open **Preferences → Nginx → Host via SSH → Open setup wizard**. Click **Generate keypair** in Step 1.

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
Host key checking always uses the configured known_hosts allow-list. If the wizard shows a new host fingerprint, verify it before trusting the key.
:::

## Step 3: Install the sudoers entry

The wizard Step 2b shows you a sudoers snippet. Copy it and install via:

```bash
sudo visudo -f /etc/sudoers.d/nginx-ui
```

Paste the snippet, save, exit. visudo will reject the file if the syntax is bad.

## Step 4: Apply ACLs for a non-root user

::: details Optional ACL commands
If your nginxui user is non-root, grant it write access to /etc/nginx:

```bash
sudo setfacl -R  -m u:nginxui:rwx /etc/nginx
sudo setfacl -dR -m u:nginxui:rwx /etc/nginx
```
:::

## Step 5: Update docker-compose

The wizard Step 2a shows a compose snippet. Merge it into your existing `docker-compose.yml`. Then:

The generated snippet sets `NGINX_UI_DISABLE_BUNDLED_NGINX=true` so the container does not start its bundled nginx service while it controls the host nginx service.

```bash
docker compose up -d --force-recreate nginx-ui
```

## Step 6: Verify the setup

Back in the wizard Step 4, click **Run verification**. Every check should pass:

::: tip Expected verification result

- ✓ same_host: machine-id matched
- ✓ ssh_connect: echo ok over ssh
- ✓ sudo_available: sudo -n true succeeded
- ✓ sudoers_coverage: all required entries present
- ✓ systemctl_is_active: active
- ✓ unit_has_execreload: ExecReload is declared
- ✓ nginx_test: configuration file ok
- ✓ config_dir_writable: /etc/nginx accessible
- ✓ log_dir_readable: /var/log/nginx/access.log readable
- ✓ pid_file_present: /var/run/nginx.pid present

:::

Click **Save** after the checks pass.

## Troubleshooting

::: details `sudo_available` fails with "sudo: a password is required"
- Check your sudoers file has `NOPASSWD:` not just `(root)`.
- Check the file has correct line continuations (`\` at line endings).
:::

::: details `ssh_connect` fails with "permission denied (publickey)"
- Verify authorized_keys has the right line, owner, and permissions.
- Check sshd_config allows `PubkeyAuthentication yes`.
:::

::: warning `same_host` warns "remote host detected"
Your `host_address` resolves to a different machine. SSH mode does **not** work cross-host; see [Manage Multi-Host Nginx with Cluster](manage-multi-host-nginx-with-cluster.md).
:::

## CLI reference

Generate a keypair for host SSH:

```bash
nginx-ui host-setup keygen --out /etc/nginx-ui/host_key
```

Print all setup snippets:

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui
```

Print only Docker or host-side snippets:

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --compose
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --host
```

Use `--json`, `--override`, or `--docker-run` when you need machine-readable output, a full compose override, or a docker run command.

Run verification against the current settings:

```bash
nginx-ui host-setup test
```

## Related docs

- [Nginx configuration reference](config-nginx.md#host-ssh-control)
- [Manage Multi-Host Nginx with Cluster](manage-multi-host-nginx-with-cluster.md)
