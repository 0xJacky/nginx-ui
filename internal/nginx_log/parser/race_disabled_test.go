//go:build !race

package parser

// raceEnabled reports whether the test binary was built with the race
// detector, so performance-oriented tests can relax their thresholds.
const raceEnabled = false
