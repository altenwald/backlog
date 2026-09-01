//go:build windows

package cli

func acquireInstanceLock(dataDir string) error {
	return nil
}

func spawnDaemon(dataDir string) error {
	return nil
}
