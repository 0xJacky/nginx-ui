---
outline: [2, 3]
---

# Catalogs

The plugin marketplace of Nginx UI lists the plugins of one or more
**catalogs**: static JSON documents that describe plugins and where their
packages are. The official catalog lives at `https://plugins.nginxui.com`, and
anyone can publish a catalog of their own that people add as a source.

## Publishing in the Official Catalog

The official catalog is built from the
[nginxui/plugins](https://github.com/nginxui/plugins) repository. To submit a
plugin:

1. Publish the plugin in a public repository, with a GitHub release whose
   assets are the signed packages.
2. Make sure `nginx-ui plugin lint` and, for a plugin with a server process,
   `nginx-ui plugin conformance` pass. The repository runs both on your
   release.
3. Open an issue with the **Submit a plugin** form, which drafts the entry
   for you, or open a pull request adding `plugins/<plugin id>.json`.

The repository's contributing guide describes the review. New releases are
picked up from your repository afterwards.

## Catalog Document

```json [index.json]
{
  "schema_version": 1,
  "name": { "en": "Example Plugins", "zh_CN": "示例插件" },
  "icon": "https://plugins.example.com/assets/icon.png",
  "updated_at": "2026-09-30T00:00:00Z",
  "plugins": [ ... ]
}
```

| Field | Meaning |
| --- | --- |
| `schema_version` | `1`. |
| `name` | Name of the catalog by language, `en` as the fallback. Shown wherever Nginx UI names the source of a plugin. |
| `icon` | Square image standing for the catalog. |
| `updated_at` | Time of this version of the document. |
| `plugins` | The entries. |

The [JSON Schema](https://github.com/nginxui/plugin-spec/blob/main/schema/catalog.schema.json)
describes every member. Unknown members are ignored.

### Where to Serve It

Serve the document at `/v1/index.json` of a site, such as
`https://plugins.example.com/v1/index.json`. Then people can add the source by
entering only `plugins.example.com`: Nginx UI looks for the catalog at
`/v1/index.json`, then at `/index.json`, and keeps the address that answered.

### Name and Icon

The catalog decides its own name. Nginx UI shows it in the marketplace and in
the source list, collapses whitespace and may shorten it to 64 characters.
Without a name, it shows the host of the address.

The icon is a PNG, SVG or WebP image, square and at least 64 pixels wide
unless it is a vector image. It is loaded under the rules for
[screenshots](#screenshots), from the catalog's own host or GitHub; an icon
elsewhere is dropped and a generic image shown.

## Entries

```json [index.json]
{
  "id": "io.github.example.mydns",
  "name": { "en": "MyDNS" },
  "description": { "en": "DNS-01 challenges through the MyDNS API." },
  "author": "example",
  "author_public_key": "RWQ...",
  "repository_url": "https://github.com/example/mydns",
  "readme_url": "https://raw.githubusercontent.com/example/mydns/main/README.md",
  "icon_url": "https://raw.githubusercontent.com/example/mydns/main/icon.png",
  "categories": ["dns01"],
  "capabilities": ["dns01"],
  "license": "MIT",
  "trust": "community",
  "stage": "production",
  "releases": [ ... ]
}
```

| Field | Meaning |
| --- | --- |
| `id` | The plugin id. |
| `name`, `description` | By language, `en` required. |
| `author` | Who publishes the plugin. |
| `author_public_key`, `trust` | See [Publisher](#publisher). |
| `homepage_url`, `repository_url`, `readme_url`, `icon_url` | Links. The readme is shown in the plugin details. |
| `screenshots` | See [Screenshots](#screenshots). |
| `categories`, `capabilities` | For filtering the marketplace. |
| `license` | SPDX identifier of the plugin's license. |
| `stage` | `production` or `beta`, a label shown next to the plugin. |
| `releases` | The releases, see below. |

Links and images of an entry are used only from `https` addresses on the
catalog's host, the package's host or GitHub.

## Releases

```json [index.json]
{
  "version": "1.2.0",
  "released_at": "2026-09-30T00:00:00Z",
  "api_version": 1,
  "min_nginx_ui_version": "2.7.0",
  "platforms": ["linux-amd64", "linux-arm64"],
  "downloads": {
    "linux-amd64": { "url": "https://github.com/example/mydns/releases/download/v1.2.0/io.github.example.mydns-1.2.0-linux-amd64.tar.gz", "sha256": "..." },
    "linux-arm64": { "url": "https://github.com/example/mydns/releases/download/v1.2.0/io.github.example.mydns-1.2.0-linux-arm64.tar.gz", "sha256": "..." }
  },
  "release_notes_url": "https://github.com/example/mydns/releases/tag/v1.2.0",
  "manifest": { ... }
}
```

| Field | Meaning |
| --- | --- |
| `version` | The plugin version of the release. |
| `api_version` | Protocol generation, `1`. |
| `min_nginx_ui_version` | As in the manifest. |
| `platforms` | Where the release installs: every key of `downloads` plus the platforms of the portable package. Empty means everywhere. |
| `downloads` | Packages by platform key, or `any` for one that runs everywhere. Each has `url` and `sha256`. |
| `download_url`, `sha256` | The portable package, the fallback for platforms `downloads` does not name. |
| `channel` | `stable`, `beta` or `dev`. See [Release Channels](#release-channels). |
| `yanked` | The release is withdrawn and no longer offered. |
| `manifest` | The `plugin.json` of the release. |

A release has `downloads`, `download_url` or both. A platform entry of
`downloads` points at the package of that platform, and an `any` entry at a
package that runs everywhere. Give every download its `sha256`.

### How Nginx UI Picks a Package

On platform `P`, Nginx UI takes `downloads[P]`, then `downloads["any"]`, then
`download_url` when `platforms` is empty or contains `P` or `any`. When
nothing matches, the release does not install on `P`. It uses the same choice
everywhere: for the newest release, for update checks, for dependencies and
for downloading packages for other nodes or offline installs.

After downloading it checks the `sha256`, then the
[signature](./signing.md), and after extracting that the manifest's id and
version match the entry and the release. The checksum shows the download is
the file the catalog meant; only the signature shows who published it.

### The Manifest Snapshot

`manifest` is the manifest of the release as a whole: its `server.executables`
lists every platform the release ships, even though each per-platform package
declares only its own. Nginx UI shows the permissions, dependencies and
platforms from it before downloading anything. Large blocks such as the
DNS-01 provider list may be left out.

## Publisher

| Field | Meaning |
| --- | --- |
| `author_public_key` | The minisign public key that signs the plugin's packages, the base64 line of the `.pub` file. A package downloaded from this entry and signed with it is `community`. |
| `trust` | `official`, `verified` or `community`: the level the catalog expects. A label for listing and filtering, never a grant. `official` is only shown for an id in the reserved `com.nginxui.*` namespace, others show as `community`. |

`author_public_key` counts only for packages downloaded from the entry that
carries it. See [Signing and Trust](./signing.md#trust-levels).

## Release Channels

Every release belongs to a channel, from the most to the least stable:
`stable`, `beta` and `dev`. Without a `channel` member, the version decides:

- a version without a prerelease part is `stable`;
- a prerelease whose first identifier is `alpha`, `dev`, `nightly`,
  `snapshot`, `canary` or `preview` (in any case) is `dev`, so
  `1.0.0-nightly.20260930` is `dev` and `1.0.0-alphabet` is not;
- any other prerelease, such as `1.0.0-beta.1` or `2.0.0-rc.1`, is `beta`.

Set `channel` to put a plain version on a less stable channel. The `stage` of
an entry does not change any channel.

::: tip
Number prereleases with dot separated numbers, such as `1.1.0-beta.10`:
identifiers compare as numbers only when they stand alone, so `1.1.0-beta10`
sorts before `1.1.0-beta9`.
:::

How people receive releases:

- Each installed plugin follows a channel, `stable` unless the person chooses
  another. Updates come from the followed channel, or from the channel of the
  installed release when that is less stable, and include every more stable
  release. Someone on `beta` also gets the stable release that ends a beta
  series.
- Installing a single beta does not change the followed channel. That
  installation gets newer betas until a stable release is installed, then
  returns to stable. A plugin whose only releases are betas works the same
  way: people get the betas and then the first stable release.
- For a plugin not yet installed, Nginx UI picks the newest stable release,
  else the newest `beta`, else the newest `dev`, and marks a plugin without a
  stable release.
- Any release that is not yanked and runs on the machine can be installed by
  version. Installing an older version is a downgrade, which only happens on
  request and comes with a warning that data written by the newer version may
  not be readable.

## Screenshots

```json [index.json]
"screenshots": [
  {
    "url": "https://raw.githubusercontent.com/example/mydns/main/docs/credentials.png",
    "dark_url": "https://raw.githubusercontent.com/example/mydns/main/docs/credentials-dark.png",
    "caption": { "en": "The credential form" }
  }
]
```

Up to eight PNG, JPEG or WebP images of the plugin in use, in display order,
each with an optional caption by language. 1280 to 1920 pixels wide at about
16:10 suits every screen. Nginx UI loads them only from `https` addresses on
the catalog's host, the package's host or GitHub, drops the rest and never
treats a missing image as a fault of the entry.

`dark_url` is optional: the same view in the dark theme. Nginx UI shows it
while the interface is dark and the `url` image otherwise, so a screenshot
without a dark image, or with one it cannot load, shows the light image in
both themes. Take both images of the same view at the same size.

## Hosting a Catalog of Your Own

A catalog is a static file, so any web server or static hosting works:

- Serve it over HTTPS at `/v1/index.json`.
- Host packages, images and the readme on the same host or on GitHub.
- Give the catalog a `name` and an `icon`.
- Sign your packages: outside developer mode, unsigned packages never install.
  Your catalog's `author_public_key` makes your packages `community`; the
  operator must also allow community plugins.

People add the catalog under **Plugins > Marketplace > Sources**. Only the
official catalog can publish partner keys.

Catalogs are merged in the order they are listed, and the first catalog that
lists a plugin is the one it installs from. The official catalog cannot be
removed. It comes first unless moved: a mirror placed above it serves the
official plugins in its place. That changes where a package downloads from,
not what it is: an id in `com.nginxui.*` installs only from a package signed
with the official key, and the marketplace marks such a plugin when another
catalog offers it.
