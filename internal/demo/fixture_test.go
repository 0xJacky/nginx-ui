package demo

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureBounds writes the access log fixture as of now and returns the
// timestamps of its oldest and newest lines.
func fixtureBounds(t *testing.T, now time.Time) (oldest, newest time.Time) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "access.log")
	require.NoError(t, WriteAccessLogFixture(path, now))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	stamp := regexp.MustCompile(`\[([^\]]+)\]`)
	parse := func(line string) time.Time {
		match := stamp.FindStringSubmatch(line)
		require.NotNil(t, match, "line without a timestamp: %q", line)
		at, err := time.Parse("02/Jan/2006:15:04:05 -0700", match[1])
		require.NoError(t, err)
		return at
	}
	return parse(lines[0]), parse(lines[len(lines)-1])
}

// The log dashboard opens on the browser-local calendar day of the newest
// entry. Whatever hour the container boots and whatever zone the browser is
// in, the fixture has to reach back to that day's midnight, or the dashboard
// opens on a day that is partly empty.
func TestAccessLogFixtureCoversTheDashboardDefaultDay(t *testing.T) {
	zones := []*time.Location{time.UTC}
	for hours := -12; hours <= 14; hours++ {
		zones = append(zones, time.FixedZone("", hours*3600))
	}
	zones = append(zones,
		time.FixedZone("", 5*3600+30*60),
		time.FixedZone("", 5*3600+45*60),
		time.FixedZone("", -(9*3600+30*60)),
	)

	day := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	for hour := range 24 {
		boot := day.Add(time.Duration(hour)*time.Hour + 19*time.Minute)
		oldest, newest := fixtureBounds(t, boot)

		for _, zone := range zones {
			local := newest.In(zone)
			midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone)
			assert.False(t, oldest.After(midnight),
				"booted %s: the local day starts %s but the log starts %s",
				boot.Format(time.RFC3339), midnight.Format(time.RFC3339), oldest.Format(time.RFC3339))
		}
	}
}
