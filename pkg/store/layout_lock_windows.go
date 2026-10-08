//go:build windows

package store

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
)

func migrationLock(dir string) (func(), error) {
	f, e := os.OpenFile(filepath.Join(dir, "backlog.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	overlap := &windows.Overlapped{}
	if e = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlap); e != nil {
		f.Close()
		return nil, e
	}
	return func() { windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, overlap); f.Close() }, nil
}
