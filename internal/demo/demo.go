// Package demo fabricates the state a public demo instance cannot obtain
// honestly: geo enrichment without a city database, liveness probes without
// reachable backends, provider APIs without credentials.
//
// The mechanism is deliberate. Subsystems expose a provider slot that defaults
// to nil, and this package is the only one that fills them, from a single call
// in internal/kernel/boot.go. A production binary never calls Install, so every
// slot stays nil and this code is unreachable rather than merely un-taken.
// Expressing fabrication as an `if demo` branch at each call site would fail
// silently when it fails; a nil slot cannot.
//
// Fabrication happens at the INPUT to the real pipeline, never at the output of
// a handler. A fabricated GeoIPService feeds the real parser, the real indexer
// and the real search layer, so facets, filters and time ranges all stay honest
// code under test.
package demo

import "github.com/0xJacky/Nginx-UI/settings"

// Enabled reports whether this node runs as a public demo.
//
// This is intended to be the only reader of settings.NodeSettings.Demo outside
// internal/middleware/demo.go and the handful of grandfathered refusal sites;
// see TestNoDemoBranchesOutsideDemoPackage.
func Enabled() bool {
	return settings.NodeSettings.Demo
}
