// Package logstore is a columnar, append-only store for parsed nginx access
// log lines. See DESIGN.md for the format and the reasoning behind it.
package logstore

import "strings"

// Row is one parsed access log line.
type Row struct {
	Timestamp      int64
	IP             string
	RegionCode     string
	Province       string
	City           string
	C1             string
	C2             string
	C3             string
	C4             string
	Method         string
	Path           string
	Protocol       string
	Status         int
	BytesSent      int64
	Referer        string
	UserAgent      string
	Browser        string
	BrowserVersion string
	OS             string
	OSVersion      string
	DeviceType     string
	RequestTime    float64
	UpstreamTime   *float64
	Raw            string
}

// Column identifies a stored column. The numeric values are part of the
// on-disk format and must never be reused.
type Column uint8

const (
	ColTimestamp Column = iota
	ColStatus
	ColBytesSent
	ColRequestTime
	ColUpstreamTime
	ColRaw
	ColIP
	ColRegionCode
	ColProvince
	ColCity
	ColC1
	ColC2
	ColC3
	ColC4
	ColMethod
	ColPath
	ColProtocol
	ColReferer
	ColUserAgent
	ColBrowser
	ColBrowserVersion
	ColOS
	ColOSVersion
	ColDeviceType

	numColumns
)

// columnNames are the field names the rest of nginx-ui already uses for the
// Bleve index, so results stay drop-in compatible.
var columnNames = [numColumns]string{
	ColTimestamp:      "timestamp",
	ColStatus:         "status",
	ColBytesSent:      "bytes_sent",
	ColRequestTime:    "request_time",
	ColUpstreamTime:   "upstream_time",
	ColRaw:            "raw",
	ColIP:             "ip",
	ColRegionCode:     "region_code",
	ColProvince:       "province",
	ColCity:           "city",
	ColC1:             "c1",
	ColC2:             "c2",
	ColC3:             "c3",
	ColC4:             "c4",
	ColMethod:         "method",
	ColPath:           "path",
	ColProtocol:       "protocol",
	ColReferer:        "referer",
	ColUserAgent:      "user_agent",
	ColBrowser:        "browser",
	ColBrowserVersion: "browser_version",
	ColOS:             "os",
	ColOSVersion:      "os_version",
	ColDeviceType:     "device_type",
}

func (c Column) String() string {
	if c < numColumns {
		return columnNames[c]
	}
	return "unknown"
}

// isDict reports whether the column is dictionary encoded.
func (c Column) isDict() bool { return c >= ColIP && c < numColumns }

// countable reports whether facets and distinct counts support the column.
func (c Column) countable() bool { return c.isDict() || c == ColStatus }

// ColumnByName resolves a field name. "path_exact" is accepted as an alias of
// "path": the store keeps one exact path value and matches substrings on it.
func ColumnByName(name string) (Column, bool) {
	if name == "path_exact" {
		return ColPath, true
	}
	for i, n := range columnNames {
		if n == name {
			return Column(i), true
		}
	}
	return 0, false
}

// dictValue returns the value of a dictionary column for a row.
func dictValue(r *Row, c Column) string {
	switch c {
	case ColIP:
		return r.IP
	case ColRegionCode:
		return r.RegionCode
	case ColProvince:
		return r.Province
	case ColCity:
		return r.City
	case ColC1:
		return r.C1
	case ColC2:
		return r.C2
	case ColC3:
		return r.C3
	case ColC4:
		return r.C4
	case ColMethod:
		return r.Method
	case ColPath:
		return r.Path
	case ColProtocol:
		return r.Protocol
	case ColReferer:
		return r.Referer
	case ColUserAgent:
		return r.UserAgent
	case ColBrowser:
		return r.Browser
	case ColBrowserVersion:
		return r.BrowserVersion
	case ColOS:
		return r.OS
	case ColOSVersion:
		return r.OSVersion
	case ColDeviceType:
		return r.DeviceType
	}
	return ""
}

func setDictValue(r *Row, c Column, v string) {
	switch c {
	case ColIP:
		r.IP = v
	case ColRegionCode:
		r.RegionCode = v
	case ColProvince:
		r.Province = v
	case ColCity:
		r.City = v
	case ColC1:
		r.C1 = v
	case ColC2:
		r.C2 = v
	case ColC3:
		r.C3 = v
	case ColC4:
		r.C4 = v
	case ColMethod:
		r.Method = v
	case ColPath:
		r.Path = v
	case ColProtocol:
		r.Protocol = v
	case ColReferer:
		r.Referer = v
	case ColUserAgent:
		r.UserAgent = v
	case ColBrowser:
		r.Browser = v
	case ColBrowserVersion:
		r.BrowserVersion = v
	case ColOS:
		r.OS = v
	case ColOSVersion:
		r.OSVersion = v
	case ColDeviceType:
		r.DeviceType = v
	}
}

// sanitizeRaw keeps the newline-separated raw column well formed.
func sanitizeRaw(s string) string {
	if strings.IndexByte(s, '\n') < 0 {
		return s
	}
	return strings.ReplaceAll(s, "\n", " ")
}
