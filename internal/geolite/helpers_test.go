package geolite

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	nginxSettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/settings"
)

// useTempConfDir points settings.ConfPath at a fresh temporary directory and
// clears the custom MMDB path, restoring both when the test ends. It returns
// the directory that GetDBPath and GetDBXZPath resolve against.
func useTempConfDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	originalConfPath := settings.ConfPath
	originalCustom := nginxSettings.NginxLogSettings.IndexCustomMMDB
	settings.ConfPath = filepath.Join(dir, "app.ini")
	nginxSettings.NginxLogSettings.IndexCustomMMDB = ""
	t.Cleanup(func() {
		settings.ConfPath = originalConfPath
		nginxSettings.NginxLogSettings.IndexCustomMMDB = originalCustom
	})

	return dir
}

func setCustomMMDB(t *testing.T, value string) {
	t.Helper()
	original := nginxSettings.NginxLogSettings.IndexCustomMMDB
	nginxSettings.NginxLogSettings.IndexCustomMMDB = value
	t.Cleanup(func() { nginxSettings.NginxLogSettings.IndexCustomMMDB = original })
}

// requireCosyError asserts that err is a cosy error carrying the same scope
// and code as want. WrapErrorWithParams returns a fresh *cosy.Error, so
// errors.Is cannot be used here.
func requireCosyError(t *testing.T, err error, want error) *cosy.Error {
	t.Helper()
	require.Error(t, err)

	got, ok := errors.AsType[*cosy.Error](err)
	require.True(t, ok, "expected *cosy.Error, got %T: %v", err, err)
	expected, ok := errors.AsType[*cosy.Error](want)
	require.True(t, ok)

	require.Equal(t, expected.Scope, got.Scope)
	require.Equal(t, expected.Code, got.Code, "got error %q", got.Error())
	return got
}

// The helpers below build a minimal MaxMind DB in memory so Search can be
// exercised without shipping a binary fixture. The format is documented at
// https://maxmind.github.io/MaxMind-DB/.

const (
	mmdbTypeString = 2
	mmdbTypeUint16 = 5
	mmdbTypeUint32 = 6
	mmdbTypeMap    = 7
	mmdbTypeUint64 = 9
	mmdbTypeArray  = 11

	mmdbRecordSize = 24
)

var mmdbMetadataMarker = []byte("\xAB\xCD\xEFMaxMind.com")

func writeMMDBControl(buf *bytes.Buffer, typ, size int) {
	var sizeBytes []byte
	ctrlSize := size
	switch {
	case size < 29:
	case size < 29+256:
		ctrlSize = 29
		sizeBytes = []byte{byte(size - 29)}
	case size < 285+65536:
		ctrlSize = 30
		rest := size - 285
		sizeBytes = []byte{byte(rest >> 8), byte(rest)}
	default:
		panic(fmt.Sprintf("mmdb test encoder: size %d too large", size))
	}

	if typ <= 7 {
		buf.WriteByte(byte(typ<<5 | ctrlSize))
	} else {
		buf.WriteByte(byte(ctrlSize))
		buf.WriteByte(byte(typ - 7))
	}
	buf.Write(sizeBytes)
}

func writeMMDBUint(buf *bytes.Buffer, typ int, value uint64) {
	var raw []byte
	for value > 0 {
		raw = append([]byte{byte(value)}, raw...)
		value >>= 8
	}
	writeMMDBControl(buf, typ, len(raw))
	buf.Write(raw)
}

func encodeMMDBValue(buf *bytes.Buffer, value any) {
	switch v := value.(type) {
	case string:
		writeMMDBControl(buf, mmdbTypeString, len(v))
		buf.WriteString(v)
	case uint16:
		writeMMDBUint(buf, mmdbTypeUint16, uint64(v))
	case uint32:
		writeMMDBUint(buf, mmdbTypeUint32, uint64(v))
	case uint64:
		writeMMDBUint(buf, mmdbTypeUint64, v)
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		writeMMDBControl(buf, mmdbTypeMap, len(keys))
		for _, k := range keys {
			encodeMMDBValue(buf, k)
			encodeMMDBValue(buf, v[k])
		}
	case []any:
		writeMMDBControl(buf, mmdbTypeArray, len(v))
		for _, item := range v {
			encodeMMDBValue(buf, item)
		}
	default:
		panic(fmt.Sprintf("mmdb test encoder: unsupported type %T", value))
	}
}

// mmdbChild is one side of a search tree node: another node, a data record,
// or no data. Build one with nodeChild, recordChild or emptyChild.
type mmdbChild struct {
	node   int
	record map[string]any
}

func nodeChild(index int) mmdbChild          { return mmdbChild{node: index} }
func recordChild(r map[string]any) mmdbChild { return mmdbChild{node: -1, record: r} }
func emptyChild() mmdbChild                  { return mmdbChild{node: -1} }

// buildMMDB encodes an IPv4 database whose search tree is given node by node,
// each node as its {left, right} children for bit 0 and bit 1.
func buildMMDB(t *testing.T, nodes [][2]mmdbChild) []byte {
	t.Helper()
	nodeCount := len(nodes)

	var data bytes.Buffer
	pointerFor := func(c mmdbChild) uint32 {
		switch {
		case c.record != nil:
			offset := data.Len()
			encodeMMDBValue(&data, c.record)
			return uint32(nodeCount + 16 + offset)
		case c.node >= 0:
			return uint32(c.node)
		default:
			return uint32(nodeCount)
		}
	}

	var tree bytes.Buffer
	for _, node := range nodes {
		for _, child := range node {
			p := pointerFor(child)
			require.Less(t, p, uint32(1<<mmdbRecordSize))
			tree.Write([]byte{byte(p >> 16), byte(p >> 8), byte(p)})
		}
	}

	var out bytes.Buffer
	out.Write(tree.Bytes())
	out.Write(make([]byte, 16)) // data section separator
	out.Write(data.Bytes())
	out.Write(mmdbMetadataMarker)
	encodeMMDBValue(&out, map[string]any{
		"binary_format_major_version": uint16(2),
		"binary_format_minor_version": uint16(0),
		"build_epoch":                 uint64(1_700_000_000),
		"database_type":               "GeoLite2-City",
		"description":                 map[string]any{"en": "nginx-ui test database"},
		"ip_version":                  uint16(4),
		"languages":                   []any{"en", "zh-CN"},
		"node_count":                  uint32(nodeCount),
		"record_size":                 uint16(mmdbRecordSize),
	})
	return out.Bytes()
}
