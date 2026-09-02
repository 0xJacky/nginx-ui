# Install on OpenWrt

Nginx UI publishes an APK v3 repository for OpenWrt 25.12 at
`https://cloud.nginxui.com/openwrt/25.12/`. Repository indexes are signed with a
project-managed key and built with official OpenWrt SDKs. The package is not
submitted to the upstream OpenWrt feeds.

## Requirements

- OpenWrt 25.12 or newer with the `apk` package manager
- A supported package architecture
- An existing Nginx installation
- At least 256 MiB of writable storage

The initial repository supports:

- `x86_64`
- `aarch64_generic`
- `aarch64_cortex-a53`
- `arm_cortex-a15_neon-vfpv4`
- `arm_cortex-a7_neon-vfpv4`
- `riscv64_generic`

Check the device architecture before installation:

```shell
apk --print-arch
```

ARMv5, ARMv6, and MIPS packages are not published in the initial repository.

## Add the repository and install

Run these commands as `root`:

```shell
install -d -m 0755 /etc/apk/keys /etc/apk/repositories.d
wget -O /etc/apk/keys/nginx-ui.pem \
  https://cloud.nginxui.com/openwrt/public-key.pem
arch="$(apk --print-arch)"
repository="https://cloud.nginxui.com/openwrt/25.12/${arch}/packages.adb"
grep -qxF "$repository" /etc/apk/repositories.d/customfeeds.list 2>/dev/null || \
  echo "$repository" >> /etc/apk/repositories.d/customfeeds.list
apk update
apk add nginx-ui
```

The package installs:

- `/usr/bin/nginx-ui`
- `/etc/nginx-ui/app.ini`
- `/etc/init.d/nginx-ui`
- `/lib/upgrade/keep.d/nginx-ui`

The default backend listens on port `9000`. Nginx UI configuration and the
procd service registration are preserved across sysupgrade.

## Service management

```shell
/etc/init.d/nginx-ui status
/etc/init.d/nginx-ui restart
logread -e nginx-ui
```

Read the first-start installation secret with:

```shell
cat /etc/nginx-ui/.install_secret
```

## Upgrade

```shell
apk update
apk upgrade nginx-ui
```

APK verifies the package hash through the signed `packages.adb` repository
index. Release packages and indexes are built before the index is uploaded, so
clients never receive an index that points to an unpublished package.
