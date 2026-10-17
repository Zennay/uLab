package webui

import (
	"errors"
	"io"
	"os"
)

const (
	maxEvidenceMetadataBytes int64 = 1 << 20
	maxEvidenceConfigBytes   int64 = 4 << 20
	maxEvidenceResultBytes   int64 = 64 << 20
)

// readEvidenceSnapshot accepts only bounded, unlinked regular files.
// Use the same opened inode for the length check and the bytes consumed.
func readEvidenceSnapshot(path string, limit int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, errors.New("evidence snapshot unavailable")
	}
	if !before.Mode().IsRegular() {
		return nil, errors.New("evidence snapshot must be a regular file")
	}
	file, err := openEvidenceFile(path)
	if err != nil {
		return nil, errors.New("cannot open evidence snapshot")
	}
	defer file.Close()

	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, errors.New("evidence snapshot identity mismatch")
	}
	if opened.Size() < 0 || opened.Size() > limit {
		return nil, errors.New("evidence snapshot exceeds size limit")
	}

	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, errors.New("cannot read evidence snapshot")
	}
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(opened, after) ||
		int64(len(data)) != opened.Size() || int64(len(data)) > limit || after.Size() != opened.Size() {
		return nil, errors.New("evidence snapshot changed during read")
	}
	current, err := os.Lstat(path)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
		return nil, errors.New("evidence snapshot path changed during read")
	}
	return data, nil
}
