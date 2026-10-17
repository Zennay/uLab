package webui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// All fixtures are locally generated compatibility results; no upgrade runs.
func makeConfigIntegrityBundle(t *testing.T, root string) string {
	t.Helper()
	writeTestBundle(t, root, "config-receipt", time.Date(2026, 10, 17, 2, 0, 0, 0, time.UTC),
		true, matrix.Result{
			TargetVersion: "v2",
			Status:        engine.StatusPassed,
			Runs: []engine.RunResult{{
				SourceVersion: "v1",
				TargetVersion: "v2",
				Status:        engine.StatusPassed,
				Phases:        phaseInventory(engine.PhasePrepare, engine.PhaseUpgrade, engine.PhaseVerify, engine.PhaseCleanup),
			}},
		})
	return filepath.Join(root, "config-receipt")
}

func TestLoadBundlesAcceptsUntouchedConfigReceipt(t *testing.T) {
	root := t.TempDir()
	makeConfigIntegrityBundle(t, root)
	bundles, err := LoadBundles(root)
	if err != nil || len(bundles) != 1 {
		t.Fatalf("original bundle should remain valid, count=%d error=%v", len(bundles), err)
	}
	if got := releaseReadiness(bundles[0]).Label; got != "READY" {
		t.Fatalf("original clean bundle status = %s", got)
	}
}

func TestLoadBundlesRejectsEditedValidPolicyInsteadOfDisplayingReady(t *testing.T) {
	root := t.TempDir()
	dir := makeConfigIntegrityBundle(t, root)
	path := filepath.Join(dir, "config.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	from, to := []byte(`"require_all_paths": true`), []byte(`"require_all_paths": false`)
	changed := bytes.Replace(original, from, to, 1)
	if bytes.Equal(changed, original) {
		t.Fatalf("could not find policy in valid fixture: %s", original)
	}
	// The resulting JSON/config is still valid. Only the frozen receipt differs.
	var parsed map[string]any
	if err := json.Unmarshal(changed, &parsed); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBundles(root); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("changed policy was trusted despite frozen config hash: %v", err)
	}
	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("modified evidence must fail closed, status=%d", response.Code)
	}
	if strings.Contains(response.Body.String(), "Release readiness") {
		t.Fatal("false release-readiness page was published")
	}
}

func TestLoadBundlesRejectsForgedConfigReceipt(t *testing.T) {
	root := t.TempDir()
	dir := makeConfigIntegrityBundle(t, root)
	path := filepath.Join(dir, "metadata.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var receipt map[string]any
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt["config_sha256"] = "sha256:" + strings.Repeat("0", 64)
	changed, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBundles(root); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("forged config receipt should be rejected: %v", err)
	}
}

func TestLoadBundlesRejectsNoncanonicalSnapshotNames(t *testing.T) {
	for _, field := range []string{"config_snapshot", "result_snapshot"} {
		t.Run(field, func(t *testing.T) {
			root := t.TempDir()
			dir := makeConfigIntegrityBundle(t, root)
			path := filepath.Join(dir, "metadata.json")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var receipt map[string]any
			if err := json.Unmarshal(raw, &receipt); err != nil {
				t.Fatal(err)
			}
			receipt[field] = "other.json"
			changed, err := json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, changed, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadBundles(root); err == nil || !strings.Contains(err.Error(), "noncanonical") {
				t.Fatalf("misidentified frozen %s was trusted: %v", field, err)
			}
		})
	}
}

// A rehashed but structurally invalid config still must not enter readiness.
func TestLoadBundlesRejectsInvalidConfigWithMatchingHash(t *testing.T) {
	root := t.TempDir()
	dir := makeConfigIntegrityBundle(t, root)
	configBytes := []byte("{}\n")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), configBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	metadataPath := filepath.Join(dir, "metadata.json")
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(configBytes)
	metadata["config_sha256"] = "sha256:" + hex.EncodeToString(hash[:])
	changed, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadataPath, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBundles(root); err == nil || !strings.Contains(err.Error(), "versions.from") {
		t.Fatalf("invalid config with matching hash accepted: %v", err)
	}
}
