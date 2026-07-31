# Install on Unraid

The official Community Applications source provides two templates in the
[`nginxui/unraid`](https://github.com/nginxui/unraid)
repository. The templates become searchable in Community Applications after
that repository has passed the Unraid **Validate** and **Scan** submission
checks.

## Standalone template

Choose **Nginx UI** when Nginx UI should run and manage its bundled Nginx
instance. The default host paths are:

| Host path | Container path |
| --- | --- |
| `/mnt/user/appdata/nginx-ui` | `/etc/nginx-ui` |
| `/mnt/user/appdata/nginx-ui/nginx` | `/etc/nginx` |
| `/mnt/user/appdata/nginx-ui/www` | `/var/www` |

Open `http://<unraid-address>:8080` after installation. The initial secret is
stored at `/mnt/user/appdata/nginx-ui/.install_secret` with the default mapping.

Unraid manages container image updates, so this template does not mount the
Docker socket and disables Nginx UI's socket-dependent self-updater.

## SWAG controller template

Choose **Nginx UI (SWAG)** when an existing LinuxServer SWAG container owns
Nginx. This template disables the bundled Nginx and exposes the Nginx UI backend
on port `9000`.

Before installing:

1. Set **SWAG container name** to the exact Docker container name. The default is
   `swag`.
2. Set **SWAG appdata** to the same host path used by the SWAG container. The
   default is `/mnt/user/appdata/swag`.
3. Keep the shared target path as `/config` in both containers. Nginx UI writes
   `/config/nginx` and reads `/config/log/nginx`; the paths must be identical in
   SWAG.

The SWAG template mounts `/var/run/docker.sock` so Nginx UI can test, reload,
and restart Nginx in the other container. Docker socket access is
root-equivalent host access. Restrict Nginx UI access, use a strong installation
secret, and never expose port `9000` directly to the internet.

## PUID and PGID

The official Nginx UI image is not a LinuxServer image and does not consume
`PUID` or `PGID`. The official templates intentionally omit both variables.
Adding them does not change the container user and is not a fix for SWAG's Nginx
configuration.
