# OIDC (OpenID Connect)

This section describes how to configure an OpenID Connect identity provider (for example, Microsoft Entra ID) as a login source for Nginx UI.

Login is done through the IdP's authorization code flow. Nginx UI opens your browser to the provider, the provider redirects back to `/api/oidc_callback` with a one-time authorization code, and Nginx UI exchanges that code for a token to identify the user.

## Endpoint
- Type: `string`

The issuer URL of the OpenID Connect provider, for example `https://login.microsoftonline.com/<tenant-id>/v2.0`.
Nginx UI fetches the provider's `/.well-known/openid-configuration` from this URL, so it must be reachable from the server.

## ClientId
- Type: `string`

The application (client) ID registered with the provider.

## ClientSecret
- Type: `string`

The application (client) secret. Keep it secret.

## RedirectUri
- Type: `string`

The URL the provider redirects to after login, for example `https://nginx-ui.example.com/`.
It must match one of the redirect URIs registered on the provider side.

## Scopes
- Type: `string`

Space-separated additional scopes. The `openid`, `profile`, and `email` scopes are always requested.
Leave empty for the defaults.

## Identifier
- Type: `string`

The token claim used to resolve the Nginx UI username, for example `email` or `sub`.
If the claim is missing, Nginx UI falls back to `email`, then `name`, then `sub`.
The user must already exist in Nginx UI; OIDC signs users in, it does not create accounts.

## Entra ID example

```ini
[oidc]
ClientId     = <application-id>
ClientSecret = <client-secret>
Endpoint     = https://login.microsoftonline.com/<tenant-id>/v2.0
RedirectUri  = https://nginx-ui.example.com/
Scopes       = openid profile email
Identifier   = email
```

Register the redirect URI in the Entra ID app registration (Authentication > Redirect URI, web platform).
Note that Entra ID authorization codes are long (around 1300 characters); Nginx UI accepts codes of any length, so no truncation or "Missing code or state" errors should occur.

The same options can be set with environment variables: `NGINX_UI_OIDC_ENDPOINT`, `NGINX_UI_OIDC_CLIENT_ID`, `NGINX_UI_OIDC_CLIENT_SECRET`, `NGINX_UI_OIDC_REDIRECT_URI`, `NGINX_UI_OIDC_SCOPES`, `NGINX_UI_OIDC_IDENTIFIER`.
