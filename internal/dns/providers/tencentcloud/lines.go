package tencentcloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

type recordLineItem struct {
	LineName string           `json:"LineName"`
	Useful   bool             `json:"Useful"`
	SubGroup []recordLineItem `json:"SubGroup"`
}

type recordLineListResponse struct {
	Response struct {
		LineList []recordLineItem `json:"LineList"`
	} `json:"Response"`
}

var _ dns.RecordLineProvider = (*provider)(nil)

func (p *provider) ListRecordLines(ctx context.Context, domain string) ([]dns.RecordLine, error) {
	var response recordLineListResponse
	if err := p.client.Call(ctx, "DescribeRecordLineCategoryList", map[string]any{
		"Domain": dns.ToASCIIName(domain),
	}, &response); err != nil {
		return nil, fmt.Errorf("tencentcloud: list record lines: %w", err)
	}

	result := make([]dns.RecordLine, 0)
	seen := make(map[string]bool)
	var appendLines func([]recordLineItem)
	appendLines = func(items []recordLineItem) {
		for _, item := range items {
			name := strings.TrimSpace(item.LineName)
			if item.Useful && name != "" && !seen[name] {
				// Keep line codes consistent with RecordLine in record reads and mutations.
				result = append(result, dns.RecordLine{Code: name, Name: name})
				seen[name] = true
			}
			appendLines(item.SubGroup)
		}
	}
	appendLines(response.Response.LineList)
	return result, nil
}
