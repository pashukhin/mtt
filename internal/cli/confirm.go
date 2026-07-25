package cli

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

// confirmRemote decides whether to fetch a remote template. --yes proceeds
// without reading. A non-TTY without --yes refuses (no silent remote fetch).
// Otherwise it reads one line and proceeds only on y/yes.
func confirmRemote(in io.Reader, isTTY, autoYes bool) (bool, error) {
	if autoYes {
		return true, nil
	}
	if !isTTY {
		return false, errors.New("refusing to fetch a remote template without confirmation; pass --yes")
	}
	line, _ := bufio.NewReader(in).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// stdinIsTTY reports whether stdin is a terminal (stdlib; no x/term dep).
func stdinIsTTY() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
