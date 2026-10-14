package tencentcloud

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

func TestListRecordLinesMapsAvailableNestedLines(t *testing.T) {
	t.Parallel()

	client := &fakeTencentDNSClient{handler: func(action string, request, response any) error {
		require.Equal(t, "DescribeRecordLineCategoryList", action)
		require.Equal(t, "example.com", request.(map[string]any)["Domain"])
		return copyJSON(response, map[string]any{"Response": map[string]any{"LineList": []any{
			map[string]any{"LineName": "默认", "Useful": true, "LineId": "0"},
			map[string]any{"LineName": "电信", "Useful": true, "SubGroup": []any{
				map[string]any{"LineName": "北京电信", "Useful": false},
				map[string]any{"LineName": "上海电信", "Useful": true},
			}},
			map[string]any{"LineName": "自定义线路", "Useful": false, "LineId": nil, "SubGroup": []any{
				map[string]any{"LineName": " 办公网 ", "Useful": true},
				map[string]any{"LineName": "自定义分组", "Useful": true, "Lines": []string{"电信", "联通"}},
			}},
			map[string]any{"LineName": " 默认 ", "Useful": true},
			map[string]any{"LineName": " ", "Useful": true, "SubGroup": []any{
				map[string]any{"LineName": "移动", "Useful": true},
			}},
			map[string]any{"LineName": nil, "Useful": true, "SubGroup": nil},
			map[string]any{"LineName": "不可用", "Useful": nil},
		}}})
	}}

	lines, err := (&provider{client: client}).ListRecordLines(t.Context(), "example.com")
	require.NoError(t, err)
	require.Equal(t, []dns.RecordLine{
		{Code: "默认", Name: "默认"},
		{Code: "电信", Name: "电信"},
		{Code: "上海电信", Name: "上海电信"},
		{Code: "办公网", Name: "办公网"},
		{Code: "自定义分组", Name: "自定义分组"},
		{Code: "移动", Name: "移动"},
	}, lines)
}

func TestListRecordLinesEmptyResponse(t *testing.T) {
	t.Parallel()

	client := &fakeTencentDNSClient{handler: func(_ string, _, response any) error {
		return copyJSON(response, recordLineListResponse{})
	}}
	lines, err := (&provider{client: client}).ListRecordLines(t.Context(), "example.com")
	require.NoError(t, err)
	require.NotNil(t, lines)
	require.Empty(t, lines)
}

func TestListRecordLinesPropagatesErrors(t *testing.T) {
	t.Parallel()

	apiErr := errors.New("permission denied")
	client := &fakeTencentDNSClient{handler: func(_ string, _, _ any) error { return apiErr }}
	lines, err := (&provider{client: client}).ListRecordLines(t.Context(), "example.com")
	require.ErrorIs(t, err, apiErr)
	require.ErrorContains(t, err, "tencentcloud: list record lines")
	require.Nil(t, lines)
}

func TestListRecordLinesSendsPunycodeDomain(t *testing.T) {
	t.Parallel()

	for _, stored := range []string{"xn--fsq.example.com", "例.example.com"} {
		client := &fakeTencentDNSClient{handler: func(_ string, request, response any) error {
			require.Equal(t, "xn--fsq.example.com", request.(map[string]any)["Domain"])
			return copyJSON(response, recordLineListResponse{})
		}}
		_, err := (&provider{client: client}).ListRecordLines(t.Context(), stored)
		require.NoError(t, err)
	}
}

func TestUpdateRecordPreservesUnspecifiedLine(t *testing.T) {
	t.Parallel()

	for _, existingLine := range []string{"电信", "办公网", ""} {
		for _, requestedLine := range []*string{nil, stringPointer(""), stringPointer("  ")} {
			client := &fakeTencentDNSClient{handler: func(action string, request, response any) error {
				switch action {
				case "DescribeRecord":
					var result describeRecordResponse
					result.Response.RecordInfo = &recordInfo{ID: 42, RecordLine: existingLine}
					return copyJSON(response, result)
				case "ModifyRecord":
					require.Equal(t, firstNonEmpty(existingLine, defaultRecordLine), request.(recordMutationRequest).RecordLine)
					return nil
				default:
					t.Fatalf("unexpected action: %s", action)
					return nil
				}
			}}
			_, err := (&provider{client: client}).UpdateRecord(t.Context(), "example.com", "42", dns.RecordInput{
				Type: "A", Name: "www", Content: "192.0.2.2", TTL: 600, Line: requestedLine,
			})
			require.NoError(t, err)
			require.Len(t, client.calls, 3)
			require.Equal(t, "DescribeRecord", client.calls[0].action)
			require.Equal(t, "ModifyRecord", client.calls[1].action)
		}
	}
}

func TestUpdateRecordDoesNotMutateWhenPreservingLineFails(t *testing.T) {
	t.Parallel()

	apiErr := errors.New("describe failed")
	client := &fakeTencentDNSClient{handler: func(action string, _, _ any) error {
		require.Equal(t, "DescribeRecord", action)
		return apiErr
	}}
	_, err := (&provider{client: client}).UpdateRecord(t.Context(), "example.com", "42", dns.RecordInput{Type: "A"})
	require.ErrorIs(t, err, apiErr)
	require.ErrorContains(t, err, "preserve record line")
	require.Len(t, client.calls, 1)
}

func TestRecordMutationDefaultsAndExplicitLines(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		line     *string
		expected string
	}{
		{nil, "默认"},
		{stringPointer(""), "默认"},
		{stringPointer(" 默认 "), "默认"},
		{stringPointer("电信"), "电信"},
		{stringPointer("办公网"), "办公网"},
	} {
		request := newRecordMutationRequest("example.com", 0, dns.RecordInput{Type: "A", Line: tc.line})
		require.Equal(t, tc.expected, request.RecordLine)
	}
}

func stringPointer(value string) *string {
	return &value
}
