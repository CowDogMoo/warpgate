//go:build !windows

/*
Copyright © 2026 Jayson Grace <jayson.e.grace@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package progress

import (
	"os"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// openTTY opens /dev/tty for reading and writing. The returned file is used
// for terminal size queries (GetSize) when the caller-supplied writer is not
// detected as a terminal. Returns nil when /dev/tty is unavailable or not a
// terminal.
func openTTY() *os.File {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil
	}
	if !term.IsTerminal(int(f.Fd())) {
		f.Close()
		return nil
	}
	return f
}

// suppressEcho disables terminal echo and canonical mode on stdin so that
// keypresses do not inject characters into the terminal output. Signal
// handling (ISIG) is preserved so Ctrl+C still sends SIGINT. Returns a
// function that restores the original terminal state, or nil if stdin is
// not a terminal.
func suppressEcho() func() {
	fd := int(os.Stdin.Fd())
	termios, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		return nil
	}

	oldTermios := *termios

	// Clear ECHO (don't echo keypresses) and ICANON (disable line buffering).
	// Keep ISIG so Ctrl+C still works.
	termios.Lflag &^= unix.ECHO | unix.ICANON
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, termios); err != nil {
		return nil
	}

	return func() {
		_ = unix.IoctlSetTermios(fd, ioctlWriteTermios, &oldTermios)
	}
}
