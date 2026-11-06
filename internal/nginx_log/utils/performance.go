package utils

import (
	"unsafe"
)

// BytesToStringUnsafe converts bytes to string without allocation.
// The caller must guarantee the byte slice is not mutated afterwards.
func BytesToStringUnsafe(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return *(*string)(unsafe.Pointer(&b))
}

// AppendInt appends the decimal representation of i to b without allocation
func AppendInt(b []byte, i int) []byte {
	// Convert int to bytes efficiently
	if i == 0 {
		return append(b, '0')
	}

	// Handle negative numbers
	if i < 0 {
		b = append(b, '-')
		i = -i
	}

	// Convert digits
	start := len(b)
	for i > 0 {
		b = append(b, byte('0'+(i%10)))
		i /= 10
	}

	// Reverse the digits
	for i, j := start, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return b
}
