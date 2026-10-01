// Package npipe reaches the named pipes plugins listen on under Windows.
package npipe

import (
	"errors"
	"strings"
)

// Prefix starts the name of every pipe on the local machine. A name such as
// \\server\pipe\x would reach another machine over SMB and hand it the
// credentials of the host, so only local names are accepted.
const Prefix = `\\.\pipe\`

// maxNameLength is the limit Windows puts on a full pipe name.
const maxNameLength = 256

// ErrUnsupported is returned by Dial outside Windows.
var ErrUnsupported = errors.New("named pipes are only available on Windows")

// ErrInvalidName is returned by Dial for a name Valid rejects.
var ErrInvalidName = errors.New("invalid named pipe name")

// Valid reports whether name is a pipe on this machine whose last part starts
// with a letter or digit and uses only letters, digits, dots, dashes and
// underscores, so path normalization cannot lead it elsewhere.
func Valid(name string) bool {
	rest, ok := strings.CutPrefix(name, Prefix)
	if !ok || rest == "" || len(name) > maxNameLength {
		return false
	}
	for i, r := range rest {
		alnum := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
		if !alnum && (i == 0 || r != '.' && r != '-' && r != '_') {
			return false
		}
	}
	return true
}
