package model

import "testing"

func TestAutoBackupGetNameSanitizesUnsafeFilenameComponents(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "spaces",
			in:   "daily backup",
			want: "daily_backup",
		},
		{
			name: "unix separators and traversal",
			in:   "../daily/backup",
			want: "daily_backup",
		},
		{
			name: "windows separators and traversal",
			in:   `..\daily\backup`,
			want: "daily_backup",
		},
		{
			name: "windows drive prefix",
			in:   `C:\temp\backup`,
			want: "C_temp_backup",
		},
		{
			name: "windows reserved characters",
			in:   `daily:backup*2026?`,
			want: "daily_backup_2026",
		},
		{
			name: "embedded traversal",
			in:   "daily..backup",
			want: "daily.backup",
		},
		{
			name: "windows reserved device",
			in:   "CON",
			want: "_CON",
		},
		{
			name: "windows reserved device with extension",
			in:   "con.txt",
			want: "_con.txt",
		},
		{
			name: "unicode letters",
			in:   "每日 备份",
			want: "每日_备份",
		},
		{
			name: "empty after sanitization",
			in:   " ../.. ",
			want: "backup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			autoBackup := &AutoBackup{Name: tt.in}
			if got := autoBackup.GetName(); got != tt.want {
				t.Fatalf("GetName() = %q, want %q", got, tt.want)
			}
		})
	}
}
