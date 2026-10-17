//go:build linux

package webui

import (
	"os"
	"syscall"
)

// The no-follow and nonblocking flags make a symlink/FIFO swap fail before
// the open can traverse an external file or block a read-only HTTP request.
func openEvidenceFile(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
