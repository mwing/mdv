package main

import (
	"bytes"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

type winsize struct{ Row, Col, X, Y uint16 }

func termSize(fd int) (ws winsize, ok bool) {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	return ws, e == 0 && ws.Col > 0
}

// Not authoritative: the terminal itself is asked before any image is sent.
// Multiplexers need a passthrough wrapping that is not implemented.
func imageTerminal() bool {
	if os.Getenv("TMUX") != "" || os.Getenv("ZELLIJ") != "" {
		return false
	}
	term := os.Getenv("TERM")
	return strings.Contains(term, "kitty") || strings.Contains(term, "ghostty") ||
		os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("TERM_PROGRAM") == "ghostty"
}

func utf8Locale() bool {
	for _, k := range [...]string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := strings.ToLower(os.Getenv(k)); v != "" {
			return strings.Contains(v, "utf-8") || strings.Contains(v, "utf8")
		}
	}
	return true
}

// XTVERSION rather than the environment: TERM and TERM_PROGRAM are inherited
// by terminals embedded in editors and by multiplexers.
func terminalName() string {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return ""
	}
	defer tty.Close()
	fd := tty.Fd()
	ioctl := func(req uintptr, t *syscall.Termios) bool {
		_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(unsafe.Pointer(t)))
		return e == 0
	}
	var saved syscall.Termios
	if !ioctl(ioctlGetTermios, &saved) {
		return ""
	}
	raw := saved
	// ISIG off too: ^C during the probe would otherwise kill the process
	// before the terminal is restored.
	raw.Lflag &^= syscall.ICANON | syscall.ECHO | syscall.ISIG
	raw.Cc[syscall.VMIN], raw.Cc[syscall.VTIME] = 0, 2
	if !ioctl(ioctlSetTermios, &raw) {
		return ""
	}
	defer ioctl(ioctlSetTermios, &saved)

	// A terminal without XTVERSION still answers the trailing
	// device-attributes request, which ends the wait before the timeout.
	if _, err := tty.WriteString("\x1b[>q\x1b[c"); err != nil {
		return ""
	}
	var reply []byte
	buf := make([]byte, 256)
	for len(reply) < 4096 {
		n, _ := syscall.Read(int(fd), buf)
		if n <= 0 {
			break
		}
		reply = append(reply, buf[:n]...)
		if i := bytes.Index(reply, []byte("\x1b[?")); i >= 0 && bytes.IndexByte(reply[i:], 'c') >= 0 {
			break
		}
	}
	return parseXTVersion(string(reply))
}

func parseXTVersion(reply string) string {
	_, rest, ok := strings.Cut(reply, "\x1bP>|")
	if !ok {
		return ""
	}
	name, _, ok := strings.Cut(rest, "\x1b\\")
	if !ok {
		return ""
	}
	return name
}

func hasPlaceholders(name string) bool {
	name = strings.ToLower(name)
	return strings.HasPrefix(name, "kitty") || strings.HasPrefix(name, "ghostty")
}
