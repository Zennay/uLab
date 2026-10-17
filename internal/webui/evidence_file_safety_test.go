package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/matrix"
)

func TestLoadBundlesRejectsSymlinkedEvidenceSnapshots(t *testing.T) {
	for _, name := range []string{"config.json", "metadata.json", "result.json"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "bundles")
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			writeTestBundle(t, root, "run", time.Now().UTC(), true, matrix.Result{
				TargetVersion: "v2",
				Status:        engine.StatusPassed,
				Runs:          []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
			})
			path := filepath.Join(root, "run", name)
			outside := filepath.Join(t.TempDir(), name)
			if err := os.Rename(path, outside); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadBundles(root); err == nil {
				t.Fatalf("evidence reader followed external symlink %s", name)
			}
		})
	}
}

func TestLoadBundlesRejectsOversizedEvidenceSnapshots(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit int64
	}{
		{"metadata.json", maxEvidenceMetadataBytes},
		{"config.json", maxEvidenceConfigBytes},
		{"result.json", maxEvidenceResultBytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "bundles")
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			writeTestBundle(t, root, "run", time.Now().UTC(), true, matrix.Result{
				TargetVersion: "v2", Status: engine.StatusPassed,
				Runs: []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
			})
			path := filepath.Join(root, "run", tc.name)
			if err := os.Truncate(path, tc.limit+1); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadBundles(root); err == nil || !strings.Contains(err.Error(), "size limit") {
				t.Fatalf("oversized %s accepted: %v", tc.name, err)
			}
		})
	}
}

func TestHandlerDoesNotRenderLinkedEvidenceOrItsLocation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "bundles")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestBundle(t, root, "run", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2", Status: engine.StatusPassed,
		Runs: []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
	})
	path := filepath.Join(root, "run", "result.json")
	outside := filepath.Join(t.TempDir(), "sensitive-local-result.json")
	if err := os.Rename(path, outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want 500 for linked evidence", response.Code)
	}
	body := response.Body.String()
	if strings.Contains(body, outside) || strings.Contains(body, "READY") || strings.Contains(body, "Release readiness") {
		t.Fatalf("external path or false READY was rendered: %s", body)
	}
}
