# Cert

## CADir
- Type: `string`
- Version：`>= v2.0.0-beta.37`

When applying for a Let's Encrypt certificate, we use the default CA address of Let's Encrypt. If you need to debug or
obtain certificates from other providers, you can set CADir to their address.

::: tip
Please note that the address provided by
CADir needs to comply with the `RFC 8555` standard.
:::

## RecursiveNameservers

- Version：`>= v2.0.0-beta.37`
- Type: `[]string`
- Example: `8.8.8.8:53,1.1.1.1:53`

This option is used to set the recursive nameservers used by
Nginx UI in the DNS challenge step of applying for a certificate.
If this option is not configured, Nginx UI will use the nameservers settings of the operating system.


## CertRenewalInterval

- Version：`>= v2.0.0-beta.37`
- Type: `int`
- Default value: `30`
- Valid range: `1-90`

This option sets the certificate's remaining-validity threshold for automatic renewal. Nginx UI attempts renewal when
the remaining validity is less than or equal to the configured number of days. The certificate worker checks every
30 minutes, but a check does not issue a new certificate before the threshold is reached.

The `RenewalInterval` name is retained for configuration compatibility. Existing configured values are preserved and
are interpreted as days of remaining validity after upgrading.

For short-lived ACME certificates, Nginx UI uses the server-provided ACME Renewal Information (ARI) schedule when
available. If ARI is unavailable, or if the configured threshold is not shorter than the certificate lifetime, Nginx UI
uses the midpoint of the certificate lifetime to avoid an immediate renewal loop.

## HTTPChallengePort

- Version：`>= v2.0.0-beta.37`
- Type: `int`
- Default: `9180`

This option is used to set the port for backend listening in the HTTP01 challenge mode when obtaining Let's Encrypt
certificates. The HTTP01 challenge is a domain validation method used by Let's Encrypt to verify that you control the
domain for which you're requesting a certificate.

## Certificate Discovery

When importing deployed certificates, automatic discovery scans the configured Nginx `ssl` directory for directories
that contain separate certificate and private key files, such as `fullchain.pem` and `privkey.pem`.

Combined certificate and private key single-PEM files are not auto-detected. Split them into separate certificate and
key files before using discovery.

## DNS Domain Management

- Version：`>= v2.2.2`
- Supported providers: Alibaba Cloud DNS, Azure DNS, Cloudflare, Huawei Cloud DNS, Tencent Cloud DNS

You can now register DNS domains inside Nginx-UI (Certificates → DNS Domains) and bind them to an existing DNS Credential.
For every registered domain the UI exposes a full DNS record management experience (list, create, update, delete) that talks directly to the provider's API.
This allows you to verify domains for certificate issuance and perform day-to-day DNS maintenance without leaving the dashboard.

> Make sure the selected DNS Credential contains the API tokens and permissions required by the provider to edit DNS records.
