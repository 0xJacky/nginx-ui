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

Point it at the Nginx UI root, not at `/api/oidc_callback`: the provider sends the browser back to the UI, and the login page then posts the code and state to `/api/oidc_callback` itself. Using the API path as the redirect URI returns 404.

## Scopes
- Type: `string`

Space-separated scopes to request. When empty, Nginx UI requests `openid profile email`.
When set, the value replaces the defaults entirely, so include `openid` yourself (and `email` or `profile` if the claim you use as Identifier needs them), for example `openid profile email`.

## Identifier
- Type: `string`

The ID token claim whose value must equal the Nginx UI username, for example `sub`, `email`, `preferred_username`, `oid`, or `upn`.
The user must already exist in Nginx UI; OIDC signs users in, it does not create accounts.
Nginx UI usernames are limited to 20 characters, so the claim value must fit in that limit.

When Identifier is set, only that claim is used. If the claim is missing, is not a string, or is empty, the login is refused and the error names the claim; there is no fallback to other claims.

When Identifier is empty, Nginx UI uses the first claim present in this order:

1. `email`, only when the provider marks it verified (see below)
2. `sub`

Display-name claims such as `name` are never used, and `preferred_username` is only used when you configure it explicitly, because many providers let end users choose or change it. Configure `preferred_username` only if your provider guarantees it is unique and controlled by the provider.

Whenever `email` is the claim that resolves the user, whether configured as Identifier or selected by default, the provider must also send `email_verified` as `true` (the string `"true"` is accepted too). Otherwise the login is refused. If `email` is present but not verified and you want to match on `sub` instead, set Identifier to `sub`.

When a login is refused or no matching user exists, the error response includes `identifier`, `resolved_by`, and `resolved_username` to help you check the mapping.

## TrustUnverifiedEmail
- Type: `bool`
- Default: `false`

Accept the `email` claim as the identifier even when the provider does not send `email_verified=true`.
Some providers, such as Microsoft Entra ID, never emit `email_verified`. Prefer a stable claim like `sub`, `oid`, or `preferred_username` for those providers.
Enable this option only if you are certain the provider does not let users set an email address they do not own: anyone who can make the provider issue a token with a matching `email` value can sign in as that Nginx UI user.

## Entra ID example

```ini
[oidc]
ClientId     = <application-id>
ClientSecret = <client-secret>
Endpoint     = https://login.microsoftonline.com/<tenant-id>/v2.0
RedirectUri  = https://nginx-ui.example.com/
Scopes       = openid profile email
Identifier   = preferred_username
```

In a single-tenant Entra ID app, `preferred_username` is usually the user principal name (for example `alice@contoso.com`). Microsoft documents it as mutable, so use it only if your tenant administrators control UPN changes; `oid` is the immutable alternative.
Entra ID does not emit `email_verified`, so `Identifier = email` is refused unless you also set `TrustUnverifiedEmail = true`; see [TrustUnverifiedEmail](#trustunverifiedemail) for the risk before doing so.

Register the redirect URI in the Entra ID app registration (Authentication > Redirect URI, web platform).
Note that Entra ID authorization codes are long (around 1300 characters); Nginx UI accepts codes of any length, so no truncation or "Missing code or state" errors should occur.

The same options can be set with environment variables: `NGINX_UI_OIDC_ENDPOINT`, `NGINX_UI_OIDC_CLIENT_ID`, `NGINX_UI_OIDC_CLIENT_SECRET`, `NGINX_UI_OIDC_REDIRECT_URI`, `NGINX_UI_OIDC_SCOPES`, `NGINX_UI_OIDC_IDENTIFIER`, `NGINX_UI_OIDC_TRUST_UNVERIFIED_EMAIL`.
