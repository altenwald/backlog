package version

import "fmt"

var (
	// Version is dynamically set via -ldflags at build time
	Version = "dev"
	// GitCommit is set via -ldflags at build time
	GitCommit = "none"
	// BuildDate is set via -ldflags at build time
	BuildDate = "unknown"
)

// String returns a short version string (e.g. "v0.1.0" or "v0.1.0 (abc1234)")
func String() string {
	if GitCommit != "" && GitCommit != "none" {
		return fmt.Sprintf("%s (%s)", Version, GitCommit)
	}
	return Version
}

// FullInfo returns multi-line version details for the version command and About dialog
func FullInfo() string {
	return fmt.Sprintf("Backlog %s\nCommit: %s\nBuilt:  %s", Version, GitCommit, BuildDate)
}
