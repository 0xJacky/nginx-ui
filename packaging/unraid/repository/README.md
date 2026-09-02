# Nginx UI for Unraid

This directory is the source for the official Nginx UI Community Applications
repository. Publish its contents at `nginxui/unraid` before submitting
that public repository to Community Applications.

## Templates

### Nginx UI

The standard template runs the bundled Nginx instance. It persists the Nginx UI
database and settings, Nginx configuration, and web root separately. Container
updates are handled by Unraid, so the template deliberately disables Nginx UI's
Docker-socket-dependent self-updater and does not mount the Docker socket.

Open the WebUI at `http://<unraid-address>:8080` after installation. The first
startup secret is stored in
`/mnt/user/appdata/nginx-ui/.install_secret` by default.

### Nginx UI (SWAG)

The SWAG template runs Nginx UI as a controller for an existing
[LinuxServer SWAG](https://github.com/linuxserver/docker-swag) container. It
disables the bundled Nginx and exposes the Nginx UI backend on port `9000`.

Before installation:

1. Keep the SWAG container name as `swag`, or change the template's **SWAG
   container name** value to the actual name.
2. Set **SWAG appdata** to the same host directory that SWAG mounts at
   `/config`. The default is `/mnt/user/appdata/swag`.
3. Keep the container target exactly `/config`. Nginx UI and SWAG must see the
   configuration and log files at identical paths.

The SWAG template mounts `/var/run/docker.sock`. Anyone who can control Nginx UI
through that socket may gain root-equivalent control of the Unraid host. Restrict
Nginx UI access, use a strong initial secret, and do not expose port `9000`
directly to the internet.

The Nginx UI image does not use LinuxServer's `PUID` or `PGID` variables. They
are intentionally absent from both templates; adding them does not change the
container user and can reintroduce invalid bundled-Nginx assumptions.

## Validation

Install `xmllint`, then run:

```shell
bash tests/validate.sh
```

After publishing the repository, also run **Validate** and **Scan** in the
[Community Applications submission flow](https://ca.unraid.net/submit).
