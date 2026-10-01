---
outline: [2, 3]
---

# Packaging

A plugin is distributed as a gzip compressed tar archive (`.tar.gz`) with
`plugin.json` at its root. A release can ship one **portable** package, one
package **per platform**, or both.

## Building a Package

`nginx-ui plugin pack` builds a package from a plugin directory, and signs it
when you pass a key:

```bash
nginx-ui plugin pack ./mydns io.github.example.mydns-1.0.0-linux-amd64.tar.gz --key mydns.key
```

Any tool that writes a tar archive works too, as long as the result follows
this page. `nginx-ui plugin lint <package>` checks it.

## Layout

- `plugin.json` is at the root of the archive, or exactly one directory down
  (the layout `tar czf x.tar.gz plugin-dir/` produces).
- The root holds `README.md`, `LICENSE` and `CHANGELOG.md`. They are for the
  person installing the plugin and for anyone auditing it: a missing
  `README.md` is an error, a missing `LICENSE` or `CHANGELOG.md` a warning.
- The package contains every file `server.executables` declares.
- It contains no symbolic or hard links.

## Limits

| Limit | Value |
| --- | --- |
| Entries (files and directories) | 10,000 |
| Total size of the files, uncompressed | 256 MiB |

Nginx UI stops extracting as soon as a package exceeds a limit. A native
plugin that ships executables for many platforms exceeds the size limit
quickly: ship one package per platform instead.

## Safe Paths

Every archive entry and every path in the manifest is a **safe relative
path**:

- `/` separates segments, never `\`;
- not empty and no NUL byte;
- not absolute, neither `/foo` nor a Windows drive such as `C:foo`;
- already clean: no `.` segments and no `//`;
- not `.` or `..`, and not starting with `../`.

Nginx UI refuses a package with any other path instead of fixing it, and makes
sure nothing it extracts ends up outside the plugin directory.

## Per-Platform Packages

A per-platform package contains the executable for one platform. Its file
name ends in that platform, and `server.executables` declares exactly that
platform:

```text
io.github.example.mydns-1.0.0-linux-arm64.tar.gz
```

```json [plugin.json]
"server": { "executables": { "linux-arm64": "dist/linux-arm64/mydns" } }
```

All packages of one release carry the same manifest except for
`server.executables`, and the same files except for the executables and the
signature files, so what a person approves is the same whichever package
their platform gets.

A portable package may declare any number of platforms and contains the
executable of every platform it declares. A package without `server`, or with
an interpreted `server.command`, runs everywhere.

See [Naming](./naming.md#package-file-names) for how file names are built.

## Executables

Nginx UI marks every file that `server.executables` or a path in
`server.command` points at as executable after extracting it, whatever the
archive says, and leaves the permission bits of other files alone. A
platform the package does not ship is left out of `server.executables` rather
than declared without its file.

## Reproducible Builds

Build packages deterministically where the toolchain allows: stable file
order, no timestamps beyond what the format needs. Then the checksum of a
package means something, and anyone can confirm a package was built from the
published source. Write `plugin.json` with sorted keys if you generate it.

## Installing

When it installs a package, Nginx UI:

1. extracts it within the limits above;
2. validates the manifest, and discards everything when it fails;
3. checks the [signature](./signing.md) and derives the trust level;
4. shows the person the plugin's id, version, permissions and trust level,
   and asks for approval;
5. installs it, replacing the previous version when there was one.
