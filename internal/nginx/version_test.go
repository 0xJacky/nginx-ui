package nginx

import "testing"

func TestParseCoreVersion(t *testing.T) {
	tests := []struct {
		output              string
		major, minor, patch int
		ok                  bool
	}{
		{"nginx version: nginx/1.25.3\nbuilt by gcc", 1, 25, 3, true},
		{"nginx version: openresty/1.21.4.3", 1, 21, 4, true},
		{"Tengine version: Tengine/3.1.0\nnginx version: nginx/1.24.0", 1, 24, 0, true},
		{"", 0, 0, 0, false},
	}
	for _, tt := range tests {
		major, minor, patch, ok := ParseCoreVersion(tt.output)
		if ok != tt.ok || major != tt.major || minor != tt.minor || patch != tt.patch {
			t.Errorf("ParseCoreVersion(%q) = %d.%d.%d %v, want %d.%d.%d %v",
				tt.output, major, minor, patch, ok, tt.major, tt.minor, tt.patch, tt.ok)
		}
	}
}
