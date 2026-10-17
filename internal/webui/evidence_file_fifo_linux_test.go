//go:build linux

package webui

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/matrix"
)

func TestLoadBundlesRejectsFifoWithoutBlocking(t *testing.T) {
	for _, name := range []string{"metadata.json", "config.json", "result.json"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "bundles")
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			writeTestBundle(t, root, "run", time.Now().UTC(), true, matrix.Result{
				TargetVersion: "v2", Status: engine.StatusPassed,
				Runs: []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
			})
			path := filepath.Join(root, "run", name)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := syscall.Mkfifo(path, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadBundles(root); err == nil || !strings.Contains(err.Error(), "regular file") {
				t.Fatalf("FIFO was opened or accepted: %v", err)
			}
		})
	}
}
