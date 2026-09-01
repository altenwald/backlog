//go:build !windows

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var lockFile *os.File

func acquireInstanceLock(dataDir string) error {
	lockPath := filepath.Join(dataDir, "backlog.lock")
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("error opening lock file: %w", err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		// Another instance holds the lock. Read its PID.
		var pid int
		_, _ = fmt.Fscanf(file, "%d", &pid)
		_ = file.Close()
		if pid > 0 {
			return fmt.Errorf("⚠️  Backlog is already running (PID: %d). Only one instance can run at a time", pid)
		}
		return errors.New("⚠️  Backlog is already running. Only one instance can run at a time")
	}

	// Truncate and write our own PID
	_ = file.Truncate(0)
	_, _ = file.Seek(0, 0)
	_, _ = fmt.Fprintf(file, "%d\n", os.Getpid())
	_ = file.Sync()

	// Retain file handle open for the entire process lifetime
	lockFile = file
	return nil
}
