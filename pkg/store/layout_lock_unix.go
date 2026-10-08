//go:build !windows

package store

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func migrationLock(dir string) (func(), error) {
	f, e := os.OpenFile(filepath.Join(dir, "backlog.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, fmt.Errorf("stop the running Backlog before migrating its data: %w", e)
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}
