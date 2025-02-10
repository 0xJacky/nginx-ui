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
PrimeWaf in the DNS challenge step of applying for a certificate.
If this option is not configured, PrimeWaf will use the nameservers settings of the operating system.


## CertRenewalInterval

- Version：`>= v2.0.0-beta.37`
- Type: `int`
- Default value: `7`

This option is used to set the automatic renewal interval of the Let's Encrypt certificate.
By default, PrimeWaf will automatically renew the certificate every 7 days.

## HTTPChallengePort

- Version：`>= v2.0.0-beta.37`
- Type: `int`
- Default: `9180`

This option is used to set the port for backend listening in the HTTP01 challenge mode when obtaining Let's Encrypt
certificates. The HTTP01 challenge is a domain validation method used by Let's Encrypt to verify that you control the
domain for which you're requesting a certificate.
