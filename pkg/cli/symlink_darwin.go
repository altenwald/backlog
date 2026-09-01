//go:build darwin

package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// ensureCLISymlink automatically creates the CLI symlink in PATH if running from an installed .app bundle
func ensureCLISymlink() {
	exe, err := os.Executable()
	if err != nil {
		return
	}

	// Only link automatically if running inside a .app bundle in /Applications or ~/Applications
	if !strings.Contains(exe, ".app/Contents/MacOS/backlog") {
		return
	}

	targets := []string{
		"/usr/local/bin/backlog",
	}
	if home, err := os.UserHomeDir(); err == nil {
		targets = append(targets, filepath.Join(home, ".local", "bin", "backlog"))
	}

	for _, target := range targets {
		dir := filepath.Dir(target)
		_ = os.MkdirAll(dir, 0755)

		// Check if already pointing to this executable
		if dest, err := os.Readlink(target); err == nil && dest == exe {
			return
		}

		_ = os.Remove(target)
		if err := os.Symlink(exe, target); err == nil {
			return
		}
	}
}
