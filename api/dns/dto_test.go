package dns

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/require"
)

func TestToDDNSResponseNormalizesIPVersion(t *testing.T) {
	t.Run("normalizes supported value", func(t *testing.T) {
		resp := toDDNSResponse(&model.DDNSConfig{IPVersion: " IPv4 "})
		require.Equal(t, "ipv4", resp.IPVersion)
	})

	t.Run("defaults unsupported value to ipv4 ipv6", func(t *testing.T) {
		resp := toDDNSResponse(&model.DDNSConfig{IPVersion: "invalid"})
		require.Equal(t, "ipv4_ipv6", resp.IPVersion)
	})

	t.Run("normalizes supported dual stack values", func(t *testing.T) {
		for _, tc := range []struct {
			input string
			want  string
		}{
			{input: " IPv4_IPv6 ", want: "ipv4_ipv6"},
			{input: "IPv6_IPv4", want: "ipv6_ipv4"},
			{input: "BOTH_REQUIRED", want: "both_required"},
		} {
			resp := toDDNSResponse(&model.DDNSConfig{IPVersion: tc.input})
			require.Equal(t, tc.want, resp.IPVersion)
		}
	})
}
