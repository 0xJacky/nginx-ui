# App

## PageSize

- Type: `int`
- Default: 10
- Version: `>=v2.0.0-beta.37`

This option is used to set the page size of list pagination in the Nginx UI. Adjusting the page size can help in
managing large amounts of data more effectively, but a too large number can increase the load on the server.

## JwtSecret
- Type: `string`
- Version: `>=v2.0.0-beta.37`

This option is used to configure the key used by the Nginx UI server to generate JWT.

JWT is a standard for verifying user identity. It can generate a token after the user logs in, and then use the token to verify the user's identity in subsequent requests.

If you use the one-click installation script to deploy Nginx UI, the script will generate a UUID value and set it as the value of this option.
