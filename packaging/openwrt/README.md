# OpenWrt APK repository

Nginx UI publishes signed APK v3 repositories for OpenWrt 25.12 at
`https://cloud.nginxui.com/openwrt/25.12/`. The packages are built with the
official OpenWrt 25.12.5 SDK and use the same release binaries as the Linux
release archives.

No package is submitted to the upstream OpenWrt package feeds. The private
repository signing key is stored only in the `OPENWRT_APK_PRIVATE_KEY` GitHub
Actions secret. CI derives and publishes the corresponding public key.

## Supported package architectures

| OpenWrt package architecture | SDK target | Nginx UI binary |
| --- | --- | --- |
| `x86_64` | `x86/64` | `linux-64` |
| `aarch64_generic` | `armsr/armv8` | `linux-arm64-v8a` |
| `aarch64_cortex-a53` | `mediatek/filogic` | `linux-arm64-v8a` |
| `arm_cortex-a15_neon-vfpv4` | `armsr/armv7` | `linux-arm32-v7a` |
| `arm_cortex-a7_neon-vfpv4` | `ipq40xx/generic` | `linux-arm32-v7a` |
| `riscv64_generic` | `sifiveu/generic` | `linux-riscv64` |

Other OpenWrt package architectures are intentionally rejected by APK until a
matching SDK build is added and tested. In particular, ARMv5, ARMv6, and MIPS
devices are not in the initial support matrix.

Nginx UI does not install or replace OpenWrt's web server. An existing Nginx
installation is required for configuration management. Because release
binaries are large, keep at least 256 MiB of writable storage available before
installation.

## Install

Run the following commands as root on OpenWrt 25.12 or newer:

```shell
install -d -m 0755 /etc/apk/keys /etc/apk/repositories.d
wget -O /etc/apk/keys/nginx-ui.pem https://cloud.nginxui.com/openwrt/public-key.pem
arch="$(apk --print-arch)"
echo "https://cloud.nginxui.com/openwrt/25.12/${arch}/packages.adb" >> /etc/apk/repositories.d/customfeeds.list
apk update
apk add nginx-ui
```

The package installs:

- `/usr/bin/nginx-ui`
- `/etc/nginx-ui/app.ini`
- `/etc/init.d/nginx-ui`
- `/lib/upgrade/keep.d/nginx-ui`

The configuration and service definition are preserved by sysupgrade. The
service is enabled through OpenWrt's standard package post-install handling.
