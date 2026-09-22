package settings

import "testing"

func TestDefaultTerminalStartCmd(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{goos: "windows", want: "cmd.exe"},
		{goos: "linux", want: "login"},
		{goos: "darwin", want: "login"},
	}

	for _, test := range tests {
		t.Run(test.goos, func(t *testing.T) {
			if got := defaultTerminalStartCmd(test.goos); got != test.want {
				t.Fatalf("defaultTerminalStartCmd(%q) = %q, want %q", test.goos, got, test.want)
			}
		})
	}
}
