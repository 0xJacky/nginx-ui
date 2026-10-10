# MFA policy and recovery

NGINX UI supports TOTP authenticators and WebAuthn passkeys. A configured, usable
passkey or TOTP enrollment satisfies the MFA requirement. Recovery codes are
one-time backup credentials; save them somewhere outside NGINX UI.

## Enforcement

The authentication settings contain two switches:

- **Require MFA for all users** applies to every account on this instance.
- **Require local MFA for SSO sign-in** also applies local MFA verification and
  required enrollment to OIDC and Casdoor sign-in. It is off by default.

User management can require MFA for an individual account. Global enforcement
takes precedence. Removing global enforcement preserves individual requirements
and existing credentials. Policy changes apply at the next sign-in and do not
end existing sessions.

The user list distinguishes policy from enrollment. A required account without
a usable factor is shown as pending enrollment. After primary authentication,
that user receives a browser-bound enrollment session valid for ten minutes.
It cannot access management APIs, terminals, or WebSockets. Completing MFA
enrollment or verification issues the normal login credentials.

Accounts with management access must verify their own MFA before changing MFA
policies or resetting another account. Service tokens and node credentials
cannot perform these operations. This feature does not introduce new roles.

## Changing authenticators

Personal settings can disable optional MFA. Enforced accounts cannot remove
their last usable factor. To replace TOTP, verify the new authenticator before
the old secret is replaced. Failed or cancelled enrollment leaves the old
authenticator active. Save the newly generated recovery codes; they replace
the previous codes.

## Resetting lost credentials

The user MFA settings provide an administrator reset. It removes TOTP, all
passkeys, and recovery codes, revokes login and short tokens, invalidates
pre-authentication and sensitive-operation sessions, and closes interactive
WebSocket connections. The user's MFA policy is preserved. Required accounts
must enroll again at their next sign-in.

If the only administrator loses every factor and recovery code, run this
command on the host using the configuration of the affected instance:

```sh
nginx-ui --config /path/to/app.ini reset-mfa --username admin
```

This command requires local access to the instance configuration and database.
It does not change the password, account status, or enforcement policy. Other
running processes detect the changed credential version and close existing
WebSocket connections within approximately one second. Authentication
credentials are excluded from HTTP audit payloads.
