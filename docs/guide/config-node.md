# Node

## Name
- Type: `string`
- Version：`>= v2.0.0-beta.37`

Use this option to customize the name of local server to be displayed in the environment indicator.


## Secret
- Type: `string`
- Version: `>= v2.0.0-beta.37`

This secret is used to authenticate the communication between the Nginx UI servers.
Also, you can use this secret to access the Nginx UI API without a password.

## SkipInstallation
- Type: `boolean`
- Version: `>= v2.0.0-beta.37`

By setting this option to `true`, you can skip the installation of the Nginx UI server.
This is particularly useful when you want to deploy Nginx UI to
multiple servers using the same configuration file or environment variables.

By default, if you enable the skip install mode but do not set the `App.JwtSecret` and `Node.Secret` options
in the server section, Nginx UI will generate a random UUID value for these two options.
