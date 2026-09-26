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
	"github.com/Zennay/ulab/internal/evidence"
	"github.com/Zennay/ulab/internal/matrix"
)

func TestHandlerRendersLatestEvidenceMatrix(t *testing.T) {
	root := t.TempDir()
	older := time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)

	writeTestBundle(t, root, "older", older, matrix.Result{
		TargetVersion: "v2",
		Status:        engine.StatusPassed,
		Runs: []engine.RunResult{{
			SourceVersion: "v1",
			TargetVersion: "v2",
			Status:        engine.StatusPassed,
		}},
	})
	writeTestBundle(t, root, "newer", newer, matrix.Result{
		TargetVersion: "v3",
		Status:        engine.StatusFailed,
		Runs: []engine.RunResult{{
			SourceVersion: "v2",
			TargetVersion: "v3",
			Status:        engine.StatusFailed,
			FailureKind:   engine.FailureHook,
			Phases: []engine.PhaseResult{{
				Phase:  engine.PhaseVerify,
				Status: engine.StatusFailed,
				Error:  "missing invariant",
			}},
		}},
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{"newer", "v2", "v3", "failed", "verify", "missing invariant", "ulab test"} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerSelectsRequestedEvidenceRun(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)
	writeTestBundle(t, root, "first", now, matrix.Result{TargetVersion: "v2", Status: engine.StatusPassed})
	writeTestBundle(t, root, "second", now.Add(time.Hour), matrix.Result{TargetVersion: "v3", Status: engine.StatusPassed})

	request := httptest.NewRequest(http.MethodGet, "/?run=first", nil)
	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "<code>first</code>") {
		t.Fatalf("response did not select requested run: %s", response.Body.String())
	}
}

func TestHandlerEscapesEvidenceText(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "safe-html", time.Now().UTC(), matrix.Result{
		TargetVersion: "v2",
		Status:        engine.StatusFailed,
		Runs: []engine.RunResult{{
			SourceVersion: "<script>alert(1)</script>",
			TargetVersion: "v2",
			Status:        engine.StatusFailed,
			Phases: []engine.PhaseResult{{
				Phase:  engine.PhaseVerify,
				Status: engine.StatusFailed,
				Error:  "<img src=x onerror=alert(1)>",
			}},
		}},
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, request)

	body := response.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") || strings.Contains(body, "<img src=x onerror=alert(1)>") {
		t.Fatalf("evidence text rendered as raw HTML: %s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, "&lt;img") {
		t.Fatalf("escaped evidence text not visible: %s", body)
	}
}

func TestLoadBundlesRejectsMismatchedInvocationID(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "bundle-id", time.Now().UTC(), matrix.Result{Status: engine.StatusPassed})

	metadataPath := filepath.Join(root, "bundle-id", "metadata.json")
	data := []byte("{\"schema_version\":1,\"invocation_id\":\"other\",\"created_at\":\"2026-09-26T20:00:00Z\"}\n")
	if err := os.WriteFile(metadataPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadBundles(root)
	if err == nil || !strings.Contains(err.Error(), "invocation id") {
		t.Fatalf("LoadBundles error = %v, want invocation id mismatch", err)
	}
}

func writeTestBundle(t *testing.T, root, id string, created time.Time, result matrix.Result) {
	t.Helper()
	_, err := evidence.WriteBundle(evidence.BundleInput{
		Root:   root,
		ID:     id,
		Config: []byte("{}\n"),
		Result: result,
		Metadata: evidence.BundleMetadata{
			CreatedAt:        created,
			ReproduceCommand: []string{"ulab", "test"},
			Jobs:             2,
			TargetVersion:    result.TargetVersion,
			ToolVersion:      "test",
			ToolCommit:       "abc123",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
