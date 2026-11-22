package nginx_log

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)
	return parsed
}

// A browser east of UTC picks the local day of the last indexed entry. When
// that day has already started locally but not yet in UTC, reading the date as
// a UTC day asks for a window that holds no entries at all.
func TestResolveDashboardRangeReadsDatesInTheClientZone(t *testing.T) {
	lastEntry := mustParseTime(t, "2026-11-19T16:59:55Z") // 00:59:55 on Nov 20 in Shanghai

	window, err := resolveDashboardRange(DashboardRequest{
		StartDate: "2026-11-20",
		EndDate:   "2026-11-20",
		Timezone:  "Asia/Shanghai",
	}, time.Now())
	require.NoError(t, err)

	assert.True(t, window.Start.Equal(mustParseTime(t, "2026-11-19T16:00:00Z")), "start %s", window.Start)
	assert.True(t, window.End.Equal(mustParseTime(t, "2026-11-20T15:59:59Z")), "end %s", window.End)
	assert.False(t, lastEntry.Before(window.Start) || lastEntry.After(window.End))
	require.NotNil(t, window.Location)
	assert.Equal(t, "Asia/Shanghai", window.Location.String())
}

func TestResolveDashboardRangeKeepsUTCDaysWithoutZone(t *testing.T) {
	for _, zone := range []string{"", "Not/AZone"} {
		window, err := resolveDashboardRange(DashboardRequest{
			StartDate: "2026-11-19",
			EndDate:   "2026-11-20",
			Timezone:  zone,
		}, time.Now())
		require.NoError(t, err, "zone %q", zone)

		assert.True(t, window.Start.Equal(mustParseTime(t, "2026-11-19T00:00:00Z")), "zone %q start %s", zone, window.Start)
		assert.True(t, window.End.Equal(mustParseTime(t, "2026-11-20T23:59:59Z")), "zone %q end %s", zone, window.End)
		assert.Nil(t, window.Location, "zone %q", zone)
	}
}

func TestResolveDashboardRangeEndsAtLocalMidnightAcrossDST(t *testing.T) {
	// 2026-11-01 is 25 hours long in New York: clocks fall back at 02:00.
	window, err := resolveDashboardRange(DashboardRequest{
		StartDate: "2026-11-01",
		EndDate:   "2026-11-01",
		Timezone:  "America/New_York",
	}, time.Now())
	require.NoError(t, err)

	assert.True(t, window.Start.Equal(mustParseTime(t, "2026-11-01T00:00:00-04:00")), "start %s", window.Start)
	assert.True(t, window.End.Equal(mustParseTime(t, "2026-11-01T23:59:59-05:00")), "end %s", window.End)
	assert.Equal(t, 25*time.Hour-time.Second, window.End.Sub(window.Start))
}

func TestResolveDashboardRangeDefaultsToLast30Days(t *testing.T) {
	now := mustParseTime(t, "2026-11-20T03:00:00Z")

	window, err := resolveDashboardRange(DashboardRequest{StartDate: "2026-11-20"}, now)
	require.NoError(t, err)

	assert.True(t, window.End.Equal(now))
	assert.True(t, window.Start.Equal(now.AddDate(0, 0, -30)))
}

func TestResolveDashboardRangeRejectsMalformedDates(t *testing.T) {
	_, err := resolveDashboardRange(DashboardRequest{StartDate: "20/11/2026", EndDate: "2026-11-20"}, time.Now())
	assert.ErrorContains(t, err, "start_date")

	_, err = resolveDashboardRange(DashboardRequest{StartDate: "2026-11-20", EndDate: "tomorrow"}, time.Now())
	assert.ErrorContains(t, err, "end_date")
}
