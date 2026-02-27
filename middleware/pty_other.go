//go:build !linux && !darwin && !freebsd && !dragonfly && !netbsd && !openbsd && !solaris

package middleware

import (
	"io"

	"github.com/charmbracelet/ssh"
)

// ptyIO returns the session itself for I/O on non-Unix platforms where
// real PTY allocation is not supported.
func ptyIO(s ssh.Session) (io.Reader, io.Writer) {
	return s, s
}
