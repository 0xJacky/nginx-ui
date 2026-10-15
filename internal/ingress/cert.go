package ingress

// newCertManager returns the certificate manager Start installs.
func newCertManager() CertManager {
	return noCerts{}
}
