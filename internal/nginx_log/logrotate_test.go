package nginx_log

import "testing"

func TestIsLogrotateFile(t *testing.T) {
	tests := []struct {
		filename    string
		baseLogName string
		expected    bool
		description string
	}{
		// Valid logrotate patterns
		{"access.log", "access.log", true, "Current log file"},
		{"access.log.1", "access.log", true, "First rotated file"},
		{"access.log.2", "access.log", true, "Second rotated file"},
		{"access.log.10", "access.log", true, "Tenth rotated file"},
		{"access.log.1.gz", "access.log", true, "First compressed rotated file"},
		{"access.log.2.gz", "access.log", true, "Second compressed rotated file"},
		{"access.log.10.gz", "access.log", true, "Tenth compressed rotated file"},
		
		// Invalid patterns that should NOT match
		{"random.gz", "access.log", false, "Random gz file"},
		{"access_20230815.gz", "access.log", false, "Date-based naming"},
		{"access.log.gz", "access.log", false, "Direct compression without number"},
		{"access.log.old", "access.log", false, "Non-numeric suffix"},
		{"access.log.1.bz2", "access.log", false, "Different compression format"},
		{"error.log", "access.log", false, "Different log type"},
		{"access.log.1.2.gz", "access.log", false, "Multiple dots in number"},
		{"access.log.a.gz", "access.log", false, "Non-numeric rotation"},
		
		// Edge cases
		{"", "access.log", false, "Empty filename"},
		{"access.log", "", false, "Empty base name"},
		{"access", "access.log", false, "Partial match"},
		{"access.log.backup", "access.log", false, "Backup suffix"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := isLogrotateFile(tt.filename, tt.baseLogName)
			if result != tt.expected {
				t.Errorf("isLogrotateFile(%q, %q) = %v, want %v", 
					tt.filename, tt.baseLogName, result, tt.expected)
			}
		})
	}
}