package utils

import (
	"strconv"
	"testing"
)

func TestBytesToStringUnsafe(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{name: "nil slice", input: nil, want: ""},
		{name: "zero length", input: []byte{}, want: ""},
		{name: "ascii", input: []byte("test string"), want: "test string"},
		{name: "utf8", input: []byte("日志分析"), want: "日志分析"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BytesToStringUnsafe(tt.input); got != tt.want {
				t.Errorf("BytesToStringUnsafe() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppendInt(t *testing.T) {
	values := []int{0, 1, 9, 10, 123, 4567, 1<<31 - 1, -1, -456, -987}

	for _, v := range values {
		t.Run(strconv.Itoa(v), func(t *testing.T) {
			got := string(AppendInt(nil, v))
			want := strconv.Itoa(v)
			if got != want {
				t.Errorf("AppendInt(nil, %d) = %q, want %q", v, got, want)
			}
		})
	}

	// Appending to an existing buffer must preserve the prefix
	buf := []byte("id-")
	buf = AppendInt(buf, 42)
	if string(buf) != "id-42" {
		t.Errorf("AppendInt with prefix = %q, want %q", string(buf), "id-42")
	}
}
