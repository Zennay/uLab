package evidence

import (
	"os"
	"path/filepath"
	"testing"
)

// The evidence UI intentionally skips dot-prefixed entries, so the writer
// must never report success for a run that readers cannot discover.
func TestWriteBundleRejectsInvisibleAndControlRunIDs(t *testing.T) {
	for _, id := range []string{
		".hidden",
		".bundle-tmp-looks-published",
		"run\nwrapped",
		"run\ttabbed",
		"run\x00nul",
		"run\x7fdelete",
		"run\u202Ereversed",
		"run\u2066isolated",
		"run\u200Dzero-width",
	} {
		t.Run("reject", func(t *testing.T) {
			root := t.TempDir()
			_, err := WriteBundle(BundleInput{
				Root: root, ID: id, Config: []byte("{}\n"), Result: struct{}{},
			})
			if err == nil {
				t.Fatalf("WriteBundle unexpectedly accepted hidden/unsafe ID %q", id)
			}
			// Validate before mkdir/temp creation, not after writing snapshots.
			entries, readErr := os.ReadDir(root)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if len(entries) != 0 {
				t.Fatalf("rejected ID %q created evidence entries: %#v", id, entries)
			}
		})
	}
}

func TestWriteBundleVisibleRunIDRemainsDiscoverable(t *testing.T) {
	root := t.TempDir()
	paths, err := WriteBundle(BundleInput{
		Root: root, ID: "run_20261017-abc123", Config: []byte("{}\n"),
		Result: map[string]any{"status": "passed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(paths.Dir) != "run_20261017-abc123" {
		t.Fatalf("unexpected evidence directory: %q", paths.Dir)
	}
	for _, path := range []string{paths.Config, paths.Result, paths.Metadata} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("published evidence snapshot %q unavailable: %v", path, err)
		}
	}
}
