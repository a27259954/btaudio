//go:build !windows

package lang

import "os"

// System returns the LANG environment variable on non-Windows systems (used
// only by tests and tooling; the app itself is Windows-only).
func System() string { return os.Getenv("LANG") }
