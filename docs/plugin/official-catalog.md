---
outline: [2, 3]
---

# Official Catalog

The official catalog at `https://plugins.nginxui.com` is the marketplace every
Nginx UI installation offers by default. Plugins are submitted, reviewed and
managed in the [Developer Center](https://portal.nginxui.com). This page
describes that process; [Catalogs](./catalog.md) describes the catalog
format, for a catalog of your own.

## Before You Submit

Your plugin needs:

- A public GitHub repository with at least one GitHub Release whose tag is a
  semantic version such as `v1.0.0`. Its assets are the packages: the
  portable `<id>-<version>.tar.gz`, one `<id>-<version>-<goos>-<goarch>.tar.gz`
  per platform, or both, each with a `.sha256` file next to it. See
  [Packaging](./packaging.md).
- A `plugin.json` that passes `nginx-ui plugin lint` and, for a plugin with a
  server process, `nginx-ui plugin conformance`. The catalog runs both on
  your release.
- An id that follows the [naming rules](./naming.md). Without a domain of
  your own, use `io.github.<owner>.<name>`. The `com.nginxui.` namespace is
  reserved for the plugins of the Nginx UI project.
- A primary key and a signing key it certified, made once with
  `nginx-ui plugin key init --id <plugin id>`, and packages signed with
  `nginx-ui plugin pack --key signing.key`. See
  [Signing Keys](./signing.md#signing-keys).

## Submitting

1. Sign in to the [Developer Center](https://portal.nginxui.com) with GitHub
   and choose **Submit**.
2. Pick the repository. The repository has to agree to be listed: you need
   admin permission on it, or the
   [NGINX UI Plugin Catalog](https://github.com/apps/nginx-ui-plugin-catalog)
   GitHub App installed on it.
3. Paste your primary public key, the base64 line of `primary.pub`. It
   becomes `author_public_key` of the entry and decides which packages are
   `community`, see [Trust Levels](./signing.md#trust-levels).
4. The checks run on your newest release and show the result together with
   what the listing will look like:
   - the entry matches the catalog schema, the id is unique and, for an
     `io.github.<owner>.<name>` id, the repository belongs to `<owner>`;
   - every package matches the sha256 GitHub records for it, lists every file
     in `plugin.sums` and carries a signature from a signing key your primary
     key certified for this plugin;
   - `lint` and `conformance` pass on the package a linux-amd64 node would
     install.
5. Submit. The change page follows the submission through its stages,
   submitted, checks, review, merged and live, and says whom it waits on. You
   can withdraw it, or retry it after a failed check once you published a
   fixed release.

A release that does not verify is left out of the catalog, and a submission
cannot be listed until its newest release verifies.

## Review

A maintainer reviews a submission once:

1. The repository agreed to be listed.
2. The name does not pass the plugin off as another one or as the Nginx UI
   project's. Names are reviewed in every language, and none may claim to be
   official.
3. The permissions and capabilities fit what the plugin does, and
   `permission_reasons` explain them.
4. The README and the screenshots are about the plugin and follow the code of
   conduct.
5. The license and the categories fit.

Once approved, the catalog is rebuilt and the plugin appears in the
marketplace of every node within an hour, or at once for a user who refreshes
the marketplace.

## After Listing

Later releases are not reviewed one by one. The catalog reads your GitHub
Releases every hour and lists a new one as soon as its packages verify.
Prereleases are listed on the [channel](./catalog.md#release-channels) their
version names, and draft releases are ignored until they are published.

To have a release listed within minutes instead, install the
[NGINX UI Plugin Catalog](https://github.com/apps/nginx-ui-plugin-catalog)
GitHub App on the plugin repository only. Publishing a release then starts a
catalog build at once. The App receives release events only, holds no
private key and cannot read your repository.

A listed release is pinned: replacing one of its packages on GitHub does not
change what nodes download, it fails the catalog build. Publish a new version
instead.

### What Your Releases Change

The listing follows the newest stable release that is not yanked, or the
newest release while there is no stable one:

| Listing | From that release |
| --- | --- |
| Name | `name` and `i18n.<language>.name` of `plugin.json`, once a maintainer approved it. |
| Description | `description` and `i18n.<language>.description` of `plugin.json`. |
| Homepage | `homepage_url` of `plugin.json`. |
| README | `README.md` at the tag of the release. |
| Icon | The file `icon_path` names in the package: PNG, WebP or SVG of at most 256 KB. |
| Screenshots | `screenshots` of `plugin.json`, see [Screenshots](./manifest.md#screenshots). |

When a release names the plugin in a way the listing does not hold yet, in
any language, the listing keeps its names until a maintainer approves the new
ones. A name holding a word such as "official", or a character that does not
show, is left out.

Descriptions are not reviewed, but one that names Nginx UI and claims to be
official in the same sentence, such as "the official Nginx UI DNS plugin", is
left out and the listing keeps the description it had. "The official
Cloudflare API" is fine.

## Managing a Listing

The page of your plugin in the Developer Center holds:

- **Store**, **Screenshots** and **Translations**: what the marketplace shows,
  with a preview drawn by the same components Nginx UI uses. Edits are sent as
  changes you follow on the change page.
- **Versions**: yank a release, or restore a yanked one. A yanked release is
  never offered to a node that does not have it, and nodes that run it are
  told to update or remove it. Do not delete the GitHub Release of a yanked
  version: nodes that installed it still need the catalog to list it.
- **Signers**: revoke a signing key that may have leaked. Every release it
  signed is yanked and nodes treat its packages as unsigned, so publish a new
  release signed with a new key. To replace a signing key that did not leak,
  run `nginx-ui plugin key rotate`; nothing changes in the catalog.
- **Access**: who else may manage the listing.

Yanking, restoring and revoking signers apply without a review. Replacing the
primary key, for one that was lost or leaked, is reviewed by a maintainer and
yanks every listed release, because their signer certificates no longer
verify.

## Partner Plugins

`verified` trust is for partner organizations the Nginx UI maintainers vouch
for. It says who published a package, not that anyone reviewed its source.

1. Apply from the page of your organization in the Developer Center. Once
   agreed, generate a minisign key pair
   (`minisign -G -p partner.pub -s partner.key`) and send only `partner.pub`.
2. The maintainers add the key to the signed partner keyring and send back a
   partner certificate: `plugin.partner` and `plugin.partner.minisig`, which
   name your organization and may carry an expiry date.
3. Put both files, unchanged, at the root of every package before writing
   `plugin.sums`, and sign `plugin.sums` with your partner key.

Nodes install such a package as `verified` when the certificate verifies
against the official key they pin, or when their copy of the keyring lists
your key. Ask for a new certificate before an expiry date passes. If the key
may have leaked, report it privately through a
[security advisory](https://github.com/0xJacky/nginx-ui/security/advisories/new):
the maintainers revoke it in the keyring, and nodes stop trusting it on their
next catalog refresh.

## Security Problems in a Listed Plugin

Report a vulnerability, a leaked credential or a license violation in a
listed plugin privately through a
[security advisory](https://github.com/0xJacky/nginx-ui/security/advisories/new),
not in a public issue. For a security problem the maintainers may yank
releases or delist the plugin without waiting for its author, and tell the
author afterwards.
