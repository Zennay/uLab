//go:build !linux

package webui

import "os"

// Lstat/SameFile checks in readEvidenceSnapshot reject linked or replaced
// snapshots on other platforms; Linux additionally prevents open-time races.
func openEvidenceFile(path string) (*os.File, error) {
	return os.Open(path)
}
