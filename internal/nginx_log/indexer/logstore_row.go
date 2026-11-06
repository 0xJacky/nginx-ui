package indexer

import "github.com/0xJacky/Nginx-UI/internal/nginx_log/logstore"

// LogDocumentToRow converts the parser's document into a logstore row. The
// file and group paths are not part of the row: the segment a row is written
// to records them.
func LogDocumentToRow(doc *LogDocument) *logstore.Row {
	return &logstore.Row{
		Timestamp:      doc.Timestamp,
		IP:             doc.IP,
		RegionCode:     doc.RegionCode,
		Province:       doc.Province,
		City:           doc.City,
		C1:             doc.C1,
		C2:             doc.C2,
		C3:             doc.C3,
		C4:             doc.C4,
		Method:         doc.Method,
		Path:           doc.Path,
		Protocol:       doc.Protocol,
		Status:         doc.Status,
		BytesSent:      doc.BytesSent,
		Referer:        doc.Referer,
		UserAgent:      doc.UserAgent,
		Browser:        doc.Browser,
		BrowserVersion: doc.BrowserVer,
		OS:             doc.OS,
		OSVersion:      doc.OSVersion,
		DeviceType:     doc.DeviceType,
		RequestTime:    doc.RequestTime,
		UpstreamTime:   doc.UpstreamTime,
		Raw:            doc.Raw,
	}
}
