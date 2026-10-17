package webui

import (
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

func TestModifiedResultCannotPromoteBlockedUpgradeToReady(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "blocked", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2",
		Status:        engine.StatusFailed,
		Runs:          []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusFailed}},
	})
	resultPath := filepath.Join(root, "blocked", "result.json")
	raw, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	var result matrix.Result
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	result.Status = engine.StatusPassed
	result.Runs[0].Status = engine.StatusPassed
	tampered, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultPath, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	bundles, err := LoadBundles(root)
	if err == nil && len(bundles) > 0 && releaseReadiness(bundles[0]).Label == "READY" {
		t.Fatal("modified result falsely promoted historical failed evidence to READY")
	}
}

func TestLegacyReceiptWithoutResultDigestCannotClaimReady(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "legacy", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2", Status: engine.StatusPassed,
		Runs: []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
	})
	metadataPath := filepath.Join(root, "legacy", "metadata.json")
	raw, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	delete(metadata, "result_sha256")
	rewritten, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadataPath, rewritten, 0o644); err != nil {
		t.Fatal(err)
	}
	bundles, err := LoadBundles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundles) != 1 {
		t.Fatalf("legacy evidence vanished, count=%d", len(bundles))
	}
	if got := releaseReadiness(bundles[0]).Label; got != "UNVERIFIED" {
		t.Fatalf("unbound historical result was treated as verified: %s", got)
	}
}

func TestForgedResultDigestRejectsValidResultBytes(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "forged", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2", Status: engine.StatusPassed,
		Runs: []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
	})
	metadataPath := filepath.Join(root, "forged", "metadata.json")
	raw, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	metadata["result_sha256"] = "sha256:" + strings.Repeat("0", 64)
	rewritten, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadataPath, rewritten, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = LoadBundles(root)
	if err == nil || !strings.Contains(err.Error(), "result: evidence snapshot hash mismatch") {
		t.Fatalf("forged result digest was accepted: %v", err)
	}
}

func TestLegacyResultRendersUnverifiedNotReady(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "old", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2", Status: engine.StatusPassed,
		Runs: []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
	})
	path := filepath.Join(root, "old", "metadata.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	delete(metadata, "result_sha256")
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("legacy inspection status %d; want 200", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, "UNVERIFIED") || !strings.Contains(body, "not recorded (legacy)") ||
		strings.Contains(body, "readiness-status\">READY") {
		t.Fatal("legacy evidence rendered as an authenticated READY receipt")
	}
}
