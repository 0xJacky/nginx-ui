package protocol

// DeployValidateParams is the payload of deploy.validate.
type DeployValidateParams struct {
	// Kind is the target kind code declared in the manifest, without any host prefix.
	Kind string `json:"kind"`
	// Config holds the field values the user filled in, keyed by field key.
	Config map[string]string `json:"config"`
}

// DeployPushParams is the payload of deploy.push.
type DeployPushParams struct {
	Kind        string            `json:"kind"`
	Config      map[string]string `json:"config"`
	Certificate DeployCertificate `json:"certificate"`
	// DryRun asks the plugin to check the target without changing anything.
	DryRun bool `json:"dry_run,omitempty"`
}

// DeployCertificate is the certificate deploy.push carries.
type DeployCertificate struct {
	Name string `json:"name"`
	// Domains lists the domains and IP addresses the certificate covers.
	Domains []string `json:"domains,omitempty"`
	// CertificatePEM is the leaf certificate.
	CertificatePEM string `json:"certificate_pem"`
	// PrivateKeyPEM is the private key of the leaf certificate.
	PrivateKeyPEM string `json:"private_key_pem"`
	// ChainPEM holds the intermediates, starting with the issuer of the leaf.
	ChainPEM string `json:"chain_pem,omitempty"`
	// NotAfter is the expiry of the leaf as an RFC 3339 timestamp.
	NotAfter string `json:"not_after,omitempty"`
}

// DeployPushResult is the reply to deploy.push.
type DeployPushResult struct {
	// Message summarizes what was done, or what a dry run would do.
	Message string `json:"message,omitempty"`
}
