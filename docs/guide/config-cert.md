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
- Default value: `7`

This option is used to set the automatic renewal interval of the Let's Encrypt certificate.
By default, Nginx UI will automatically renew the certificate every 7 days.

## HTTPChallengePort

- Version：`>= v2.0.0-beta.37`
- Type: `int`
- Default: `9180`

This option is used to set the port for backend listening in the HTTP01 challenge mode when obtaining Let's Encrypt
certificates. The HTTP01 challenge is a domain validation method used by Let's Encrypt to verify that you control the
domain for which you're requesting a certificate.

## DNS Domain Management

- Version：`>= v2.2.2`
- Supported providers: Alibaba Cloud DNS, Tencent Cloud DNS, Cloudflare

You can now register DNS domains inside Nginx-UI (Certificates → DNS Domains) and bind them to an existing DNS Credential.
For every registered domain the UI exposes a full DNS record management experience (list, create, update, delete) that talks directly to the provider's API.
This allows you to verify domains for certificate issuance and perform day-to-day DNS maintenance without leaving the dashboard.

> Make sure the selected DNS Credential contains the API tokens and permissions required by the provider to edit DNS records.
