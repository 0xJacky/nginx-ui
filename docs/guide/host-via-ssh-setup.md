# Host SSH Setup — Walkthrough

This page walks through configuring Nginx UI (running in Docker) to manage an nginx instance installed natively on the same host.

## Prerequisites

- Linux host with nginx installed and running under systemd
- Docker installed on the same host
- An unprivileged user dedicated to Nginx UI (we use `nginxui` in examples)

## Step 1 — Create the unprivileged user

```bash
sudo useradd -r -s /bin/bash -m -G adm nginxui
```

`-G adm` grants the user read access to /var/log files including nginx logs.

## Step 2 — Generate the keypair via Nginx UI

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

## Step 3 — Install sudoers entry

The wizard Step 2b shows you a sudoers snippet. Copy it and install via:

```bash
sudo visudo -f /etc/sudoers.d/nginx-ui
```

Paste the snippet, save, exit. visudo will reject the file if the syntax is bad.

## Step 4 — Apply ACLs (optional, for non-root user)

If your nginxui user is non-root, grant it write access to /etc/nginx:

```bash
sudo setfacl -R  -m u:nginxui:rwx /etc/nginx
sudo setfacl -dR -m u:nginxui:rwx /etc/nginx
```

## Step 5 — Update your docker-compose

The wizard Step 2a shows a compose snippet. Merge it into your existing `docker-compose.yml`. Then:

```bash
docker compose up -d --force-recreate nginx-ui
```

## Step 6 — Verify

Back in the wizard Step 4, click **Run verification**. Every check should pass:

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

Click **Save** and you're done.

## Troubleshooting

**`sudo_available` fails with "sudo: a password is required"**
- Check your sudoers file has `NOPASSWD:` not just `(root)`.
- Check the file has correct line continuations (`\` at line endings).

**`ssh_connect` fails with "permission denied (publickey)"**
- Verify authorized_keys has the right line, owner, and permissions.
- Check sshd_config allows `PubkeyAuthentication yes`.

**`same_host` warns "remote host detected"**
- Your `host_address` resolves to a different machine. SSH mode does NOT work cross-host; see [Cluster Node cross-host guide](cluster-node-cross-host.md).
