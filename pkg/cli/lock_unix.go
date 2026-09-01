//go:build !windows

package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
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

func spawnDaemon(dataDir string) error {
	// First probe if another instance is already running
	lockPath := filepath.Join(dataDir, "backlog.lock")
	if f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600); err == nil {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			var pid int
			_, _ = fmt.Fscanf(f, "%d", &pid)
			_ = f.Close()
			if pid > 0 {
				return fmt.Errorf("⚠️  Backlog is already running (PID: %d). Only one instance can run at a time", pid)
			}
			return errors.New("⚠️  Backlog is already running. Only one instance can run at a time")
		}
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}

	var childArgs []string
	for _, arg := range os.Args[1:] {
		if arg == "-d" || arg == "--daemon" {
			continue
		}
		childArgs = append(childArgs, arg)
	}

	executable, err := os.Executable()
	if err != nil {
		executable = os.Args[0]
	}

	child := exec.Command(executable, childArgs...)
	child.Stdout = nil
	child.Stderr = nil
	child.Stdin = nil
	child.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := child.Start(); err != nil {
		return fmt.Errorf("failed to start background daemon: %w", err)
	}

	fmt.Printf("✔ Backlog started in background (PID: %d)\n", child.Process.Pid)
	os.Exit(0)
	return nil
}
