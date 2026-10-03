---
outline: [2, 3]
---

# Signing and Trust

A package carries its own signature, so Nginx UI can tell who published it
wherever it came from: a catalog, an upload, the offline package directory or
another node of a cluster. The signer decides the package's **trust level**,
which decides whether and how Nginx UI installs it.

Signatures use [minisign](https://jedisct1.github.io/minisign/).

## Signing a Package

Create your keys once, see [Signing Keys](#signing-keys) for what each file is
for:

```bash
nginx-ui plugin key init --id io.github.example.mydns
```

Then sign each package with the signing key when every other file of it is
final:

```bash
nginx-ui plugin pack ./mydns io.github.example.mydns-1.0.0.tar.gz --key signing.key
# or sign an existing package in place
nginx-ui plugin sign io.github.example.mydns-1.0.0.tar.gz --key signing.key
```

Both commands pack the signing key's certificate, which they find in the plugin
directory, in `--signer` or next to the key. They refuse a certificate of
another plugin or another key, which is what signing with the primary key by
mistake looks like. An encrypted secret key is read with the password in
`NGINX_UI_PLUGIN_SIGN_PASSWORD`.

Signing adds two files at the package root:

| File | Content |
| --- | --- |
| `plugin.sums` | The SHA-256 of every other file of the package. |
| `plugin.sums.minisig` | The minisign signature of `plugin.sums`. |

Any later change to a file needs a new signature. Every package of a release
is signed on its own.

### Signing by Hand

`plugin.sums` has one line per regular file, the same format `sha256sum`
prints:

```text
<sha256>  <path>
```

- 64 lowercase hex digits, two spaces, then the path relative to the package
  root with `/` separators;
- every line ends with a line feed; no carriage returns, empty lines, byte
  order mark or comments;
- sorted by path in byte order (`LC_ALL=C sort`), each path once;
- every regular file is listed, empty ones included, except `plugin.sums` and
  `plugin.sums.minisig` at the root. Directories are not listed.

Then sign it with `minisign -S -m plugin.sums`. Both minisign signature
algorithms are accepted.

## How Nginx UI Checks a Package

| The package has | Result |
| --- | --- |
| No signature, or only one of the two files | Unsigned |
| A signature by a key Nginx UI does not know | Unsigned |
| A signature that does not parse, or does not verify with the key it names | **Invalid** |
| A valid signature, but `plugin.sums` does not match the files | **Invalid** |
| A valid signature and matching files | Signed by that key |

::: danger
An invalid package changed after it was signed. Nginx UI refuses it always,
even in developer mode. `plugin.sums` matches when it lists exactly the
package's regular files with their correct SHA-256.
:::

Nginx UI checks when it shows the package before installing and again when
it installs it, for every source. A matching catalog checksum never replaces
the signature check: it only shows the download is the file the catalog
meant.

## Trust Levels

| Signer | Level |
| --- | --- |
| The official plugin key of the Nginx UI project | `official` |
| A partner key the project vouches for, and not revoked | `verified` |
| The `author_public_key` of the catalog entry the package came from, a key on the Nginx UI **Trusted Publishers** list, or a signing key one of them certified | `community` |
| No signature, or an unknown signer | `unsigned` |

The levels rank `unsigned` < `community` < `verified` < `official`. Neither a
catalog nor an operator can raise a key above `community`. The `trust` label
of a catalog entry is for display only and never grants a level.

The official plugin key is a key of its own, apart from the key that signs
Nginx UI releases: it signs plugins, partner certificates and the partner
keyring, but never an Nginx UI upgrade.

What each level allows:

- **`official` and `verified`** install normally and may update automatically.
- **`community`** installs only when the operator allows community plugins,
  and only after the person confirms that the package is signed by its author
  rather than the Nginx UI project. A package uploaded or copied into the
  offline directory is `community` only when its key is on the Trusted
  Publishers list.
- **`unsigned`** installs only in developer mode.

Nginx UI records the level and the signer's key id of every installed plugin
and shows them. An update whose package ranks below the installed plugin is a
downgrade: automatic updates refuse it, and a person starting it is warned
first. Plugins Nginx UI installs on its own, such as the DNS-01 plugin, must
be `official`.

## Publishing as a Community Author

Most plugins are community plugins. Their catalog entry names the author's
**primary key**, and the packages are signed with a **signing key** that the
primary key certifies. The primary key stays offline and practically never
changes. A signing key can live in CI and can be replaced without touching the
catalog. The official catalog lists a community package only when it is signed
this way.

1. Create the keys and certify the signing key, see
   [Signing Keys](#signing-keys).
2. Sign every package with the signing key.
3. Put the primary public key (the base64 line of its `.pub` file) into the
   catalog entry as `author_public_key`, see [Catalogs](./catalog.md#publisher).
   A package downloaded from that entry and signed by that key, or by a signing
   key it certified, is `community`.
4. People who install your package without the catalog add your primary
   public key to **Preferences > Plugins > Trusted Publishers**.

### Signing Keys

A package signed with a signing key carries its certificate, two files at the
package root:

| File | Content |
| --- | --- |
| `plugin.signer` | The signing key's minisign public key, as in a `.pub` file. |
| `plugin.signer.minisig` | A signature of that file by the primary key. |

The trusted comment of the signature names the one plugin the key may sign:

```text
signer:io.github.example.mydns
```

`nginx-ui plugin key init --id <plugin id>` writes everything once:

| File | Where it goes |
| --- | --- |
| `primary.key` | Offline storage or a password manager, with a backup. Never into a repository or CI. |
| `primary.pub` | The base64 line is the `author_public_key` of the catalog entry. |
| `signing.key` | The secrets of your CI. |
| `signing.pub` | Nowhere in particular, the certificate holds it. |
| `plugin.signer`, `plugin.signer.minisig` | Next to the plugin sources, `pack` and `sign` put them into every package. |

Secret keys are written unencrypted unless `NGINX_UI_PLUGIN_PRIMARY_PASSWORD`
or `NGINX_UI_PLUGIN_SIGN_PASSWORD` holds a password, and readable by their
owner only. `minisign -C -s primary.key` adds a password later.

The same files can be made with minisign alone:

```bash
minisign -G -p primary.pub -s primary.key
minisign -G -p signing.pub -s signing.key
cp signing.pub plugin.signer
minisign -S -m plugin.signer -x plugin.signer.minisig -s primary.key \
  -t "signer:io.github.example.mydns"
```

Both certificate files go unchanged into every package before it is signed
with `signing.key`, so `plugin.sums` lists them. One signing key can sign
several plugins, with a certificate for each:
`nginx-ui plugin key certify signing.pub --id <other plugin id> --primary primary.key`.

Nginx UI accepts a certificate when a primary key it trusts for the package
verifies it, the comment names the plugin of the package and the catalog entry
does not list the signing key in `revoked_signers`. The package is then
`community` and recorded under the primary key. A certificate that fails gives
no trust on its own: the package falls back to the key that signed it.

### Replacing and Revoking a Signing Key

To replace a signing key, for a new machine or as a routine, create a new one
certified by the primary key and sign the next release with it:

```bash
nginx-ui plugin key rotate --id io.github.example.mydns --primary primary.key --force
```

The catalog entry does not change, and older releases keep verifying with the
certificates they carry.

When a signing key may have leaked, add its key id to `revoked_signers` of the
catalog entry; `nginx-ui plugin key revoke signing.pub` prints the line. Nginx UI no longer trusts the packages it signed, and the
catalog marks the releases it signed as yanked, so nodes that run one are asked
to update.

The primary key cannot be replaced this way. A new `author_public_key` leaves
every older release without a trusted signature, so keep the primary key
offline and backed up.

## Partner Plugins

Organizations that partner with the Nginx UI project sign with their own key,
which the project vouches for with its official plugin key. Their packages derive
`verified` on any Nginx UI, even one that has never heard of the partner.

### The Partner Certificate

A partner package carries a certificate for its key, two files at the package
root:

| File | Content |
| --- | --- |
| `plugin.partner` | The partner's minisign public key, as in a `.pub` file. |
| `plugin.partner.minisig` | A signature of that file by the official plugin key of the project. |

The trusted comment of the signature names the partner and, optionally, the
last valid day of the certificate:

```text
partner:example-corp
partner:example-corp;expires:2027-09-30
```

The name uses letters, digits, dots and hyphens. The date is a UTC calendar
date: the certificate is valid through the end of that day. The project
issues certificates with `nginx-ui plugin certify` and gives them an expiry
date.

A partner copies both files unchanged into every package before signing it,
so `plugin.sums` lists them. One certificate serves every package signed with
that key; a renewed certificate is a changed file and needs a new signature.

Nginx UI accepts a certificate when `plugin.partner` is a valid public key, the
official plugin key verifies the signature over it, the comment has the format above,
it has not expired and the key is not revoked. A certificate that fails gives
no partner trust but never makes a package invalid on its own: the package
falls back to its other sources of trust.

### The Partner Keyring

The project also publishes a signed keyring next to the official catalog,
`https://plugins.nginxui.com/v1/partners.json`, listing partner keys and
revoked key ids. Nginx UI refreshes it with its catalogs, keeps the last good
copy on disk and never accepts an older one. A key the keyring lists derives
`verified` even without a certificate, and a key it revokes never derives
`verified` again, whatever certificate a package carries. Revocation does not
change the level of plugins already installed, but updates signed by the
revoked key are refused as downgrades.

## Developer Mode

::: warning
Developer mode, under **Preferences > Plugins**, is off by default. While it
is on, Nginx UI installs unsigned packages from every source and marks their
publisher as unconfirmed. It never admits an invalid package, never lifts the
community policy and never makes an unsigned plugin eligible for automatic
updates. Turn it on only while developing.
:::
