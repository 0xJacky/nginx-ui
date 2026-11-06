package protocol_test

import (
	"encoding/json"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

func TestByteSizeDecodesAnyWholeNumber(t *testing.T) {
	for raw, want := range map[string]protocol.ByteSize{
		"5242880":          5242880,
		"5242880.0":        5242880,
		"6.442450944e+09":  6442450944,
		"0":                0,
		"9007199254740992": 1 << 53,
	} {
		var got protocol.ByteSize
		if err := json.Unmarshal([]byte(raw), &got); err != nil || got != want {
			t.Errorf("%s decoded to %d (%v), want %d", raw, got, err, want)
		}
	}
	for _, raw := range []string{"-1", "1.5", `"12"`, "1e300"} {
		var got protocol.ByteSize
		if err := json.Unmarshal([]byte(raw), &got); err == nil {
			t.Errorf("%s decoded to %d, want an error", raw, got)
		}
	}

	encoded, err := json.Marshal(protocol.StorageSizeResult{Size: 6 << 30})
	if err != nil || string(encoded) != `{"size":6442450944}` {
		t.Fatalf("encoded %s (%v)", encoded, err)
	}
}
