# Webauthn
Webauthn is a web standard for secure authentication. It allows users to log in to websites using biometrics, mobile devices, and FIDO security keys. 
Webauthn is a passwordless authentication method that provides a secure and easy-to-use alternative to passwords.

Since `v2.0.0-beta.34`, PrimeWaf has supported Webauthn passkey as a login and 2FA method.

## Passkey
Passkeys are webauthn credentials that validate your identity using touch, facial recognition, a device password, or a PIN. They can be used as a password replacement or as a 2FA method.

## Configurations
To ensure security, Webauthn configuration cannot be added through the UI.

Please manually configure the following in the app.ini configuration file and restart PrimeWaf.

### RPDisplayName
- Type: `string`

This option is used to set the display name of the relying party (RP) when registering a new credential.

### RPID
- Type: `string`

This option is used to set the ID of the relying party (RP) when registering a new credential.

### RPOrigins
- Type: `[]string`

This option is used to set the origins of the relying party (RP) when registering a new credential.


Afterward, refresh this page and click add passkey again.

Due to the security policies of some browsers, you cannot use passkeys on non-HTTPS websites, except when running on `localhost`.

## Detail
1. **Automatic 2FA with Passkey:**
   When you log in using a passkey, all subsequent actions requiring 2FA will automatically use the passkey. This means you won’t need to manually click “Authenticate with a passkey” in the 2FA dialog box.
2. **Passkey Deletion:**
   If you log in using a passkey and then navigate to Settings > Authentication and delete the current passkey, the passkey will no longer be used for subsequent 2FA challenges during the current session. If Time-based One-Time Password (TOTP) is configured, it will be used instead; if not, 2FA will not be triggered.
3. **Adding a New Passkey:**
   If you log in without using a passkey and then add a new passkey via Settings > Authentication, the newly added passkey will be prioritized for all subsequent 2FA actions during the current session.
