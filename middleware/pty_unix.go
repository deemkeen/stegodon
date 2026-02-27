//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package middleware

import (
	"io"

	"github.com/charmbracelet/ssh"
)

// ptyIO returns the appropriate I/O for bubbletea based on the session's PTY.
// On Unix, if a real PTY is allocated (not emulated), the PTY slave fd is used
// for both input and output. This is required for proper terminal semantics.
func ptyIO(s ssh.Session) (io.Reader, io.Writer) {
	pty, _, ok := s.Pty()
	if !ok || s.EmulatedPty() {
		return s, s
	}
	return pty.Slave, pty.Slave
}
