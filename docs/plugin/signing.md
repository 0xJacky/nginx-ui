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

Create a key pair once and keep the secret key safe:

```bash
minisign -G -p mydns.pub -s mydns.key
```

Then sign each package when every other file of it is final:

```bash
nginx-ui plugin pack ./mydns io.github.example.mydns-1.0.0.tar.gz --key mydns.key
# or sign an existing package in place
nginx-ui plugin sign io.github.example.mydns-1.0.0.tar.gz --key mydns.key
```

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
| The `author_public_key` of the catalog entry the package came from, or a key on the Nginx UI **Trusted Publishers** list | `community` |
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

Most plugins are community plugins. To publish one:

1. Sign every package with your key.
2. Put your public key into the catalog entry as `author_public_key` (the
   base64 line of the `.pub` file), see [Catalogs](./catalog.md#publisher).
   A package downloaded from that entry and signed by that key is `community`.
3. People who install your package without the catalog add your public key to
   **Preferences > Plugins > Trusted Publishers**.

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
