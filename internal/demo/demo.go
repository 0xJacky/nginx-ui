// Package demo fabricates the state a public demo instance cannot obtain
// honestly: liveness probes without reachable backends, provider APIs without
// credentials, and a synthetic access log.
//
// The mechanism is deliberate. Subsystems expose a provider slot that defaults
// to nil, and this package is the only one that fills them, from a single call
// in internal/kernel/boot.go. A production binary never calls Install, so every
// slot stays nil and this code is unreachable rather than merely un-taken.
// Expressing fabrication as an `if demo` branch at each call site would fail
// silently when it fails; a nil slot cannot.
//
// Fabrication happens at the INPUT to the real pipeline, never at the output of
// a handler. The synthetic access log is a real file that the log viewer and
// the log analytics plugin read like any other.
package demo

import (
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
)

// demoBuildTime anchors every fabricated timestamp so the demo does not appear
// to have been built moments ago on each cold start.
var demoBuildTime = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

// Enabled reports whether this node runs as a public demo.
//
// This is intended to be the only reader of settings.NodeSettings.Demo outside
// internal/middleware/demo.go and the handful of grandfathered refusal sites;
// see TestNoDemoBranchesOutsideDemoPackage.
func Enabled() bool {
	return settings.NodeSettings.Demo
}
