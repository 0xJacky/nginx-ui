package nginx_log

import (
	"fmt"
	"time"
	// The dashboard reads its dates in the browser's IANA zone. Windows and
	// minimal containers have no zoneinfo on disk, so ship Go's copy.
	_ "time/tzdata"

	"github.com/uozi-tech/cosy/logger"
)

const dashboardDateLayout = "2006-01-02"

// dashboardRange is the resolved query window for the dashboard.
type dashboardRange struct {
	Start time.Time
	End   time.Time
	// Location is the zone the dates were read in, or nil when the client
	// named none and the legacy UTC reading applied.
	Location *time.Location
}

// resolveDashboardRange turns the request's calendar days into a time window.
//
// The dates are local to the zone the client names. A client that names none,
// such as an older frontend or a direct API caller, gets the original reading
// as UTC days. An unknown zone falls back to that too, since a dashboard read
// in UTC is more useful than an error.
//
// The window runs from the start day's midnight to the last second of the end
// day. Without both dates it covers the 30 days before now.
func resolveDashboardRange(req DashboardRequest, now time.Time) (dashboardRange, error) {
	var location *time.Location
	if req.Timezone != "" {
		loc, err := time.LoadLocation(req.Timezone)
		if err != nil {
			logger.Warnf("Dashboard: unknown timezone %q, reading the dates as UTC: %v", req.Timezone, err)
		} else {
			location = loc
		}
	}

	parseIn := time.UTC
	if location != nil {
		parseIn = location
	}

	var start, end time.Time
	if req.StartDate != "" {
		day, err := time.ParseInLocation(dashboardDateLayout, req.StartDate, parseIn)
		if err != nil {
			return dashboardRange{}, fmt.Errorf("invalid start_date format, expected YYYY-MM-DD: %w", err)
		}
		start = day
	}

	if req.EndDate != "" {
		day, err := time.ParseInLocation(dashboardDateLayout, req.EndDate, parseIn)
		if err != nil {
			return dashboardRange{}, fmt.Errorf("invalid end_date format, expected YYYY-MM-DD: %w", err)
		}
		// AddDate keeps local midnight across DST changes, where adding
		// 24 hours would not.
		end = day.AddDate(0, 0, 1).Add(-time.Second)
	}

	if start.IsZero() || end.IsZero() {
		end = now
		start = now.AddDate(0, 0, -30)
	}

	return dashboardRange{Start: start, End: end, Location: location}, nil
}
