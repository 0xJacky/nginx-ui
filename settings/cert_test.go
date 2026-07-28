package settings

import "testing"

func TestGetCertRenewalInterval(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		expected int
	}{
		{name: "minimum", value: 0, expected: 1},
		{name: "configured threshold", value: 30, expected: 30},
		{name: "maximum", value: 91, expected: 90},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := &Cert{RenewalInterval: test.value}
			if got := settings.GetCertRenewalInterval(); got != test.expected {
				t.Fatalf("GetCertRenewalInterval() = %d, want %d", got, test.expected)
			}
		})
	}
}
