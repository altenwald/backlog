package cli

import (
	"os"
	"strings"
	"testing"
)

func TestSingleInstanceLock(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "backlog-lock-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Acquire first lock
	err = acquireInstanceLock(tempDir)
	if err != nil {
		t.Fatalf("expected first acquire to succeed, got: %v", err)
	}
	defer func() {
		if lockFile != nil {
			_ = lockFile.Close()
			lockFile = nil
		}
	}()

	// 2. Attempt to acquire second lock on the same directory (simulate 2nd instance)
	err = acquireInstanceLock(tempDir)
	if err == nil {
		t.Fatal("expected second acquire to fail because instance is already running")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Fatalf("expected 'already running' error, got: %v", err)
	}

	// 3. Release first lock
	if lockFile != nil {
		_ = lockFile.Close()
		lockFile = nil
	}

	// 4. Acquire again after release
	err = acquireInstanceLock(tempDir)
	if err != nil {
		t.Fatalf("expected acquire after release to succeed, got: %v", err)
	}
}
