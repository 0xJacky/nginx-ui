package settings

// RedactedSensitiveValue is the sentinel string returned in place of
// sensitive setting values (secrets, tokens, IP allow-list entries) by the
// settings API. Payloads that echo it back are treated as "keep the current
// value" rather than an actual user-supplied override.
const RedactedSensitiveValue = "__NGINX_UI_REDACTED__"
