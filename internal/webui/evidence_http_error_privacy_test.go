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

func TestEvidenceHTTPErrorDoesNotDiscloseUntrustedMetadata(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "sample", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2",
		Status:        engine.StatusPassed,
		Runs:          []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
	})
	marker := "synthetic-private-evidence-id-9841"
	metadata := []byte("{\"schema_version\":1,\"invocation_id\":\"" + marker + "\",\"created_at\":\"2026-10-17T10:00:00Z\"}")
	if err := os.WriteFile(filepath.Join(root, "sample", "metadata.json"), metadata, 0o600); err != nil {
		t.Fatal(err)
	}
	// The internal diagnostic remains actionable for operators and tests.
	if _, err := LoadBundles(root); err == nil || !strings.Contains(err.Error(), marker) {
		t.Fatalf("expected source identity diagnostic, got %v", err)
	}
	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", response.Code)
	}
	if got := response.Body.String(); got != "evidence unavailable\n" {
		t.Fatalf("server error disclosed untrusted metadata or path: %q", got)
	}
	if !strings.HasPrefix(response.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("unexpected evidence failure content type: %q", response.Header().Get("Content-Type"))
	}
}

func TestEvidenceHTTPErrorDoesNotDiscloseLocalFilesystemRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "synthetic-private-evidence-root-1945")
	if err := os.WriteFile(root, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBundles(root); err == nil || !strings.Contains(err.Error(), "synthetic-private-evidence-root-1945") {
		t.Fatalf("expected internal filesystem error with path, got %v", err)
	}
	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", response.Code)
	}
	if got := response.Body.String(); got != "evidence unavailable\n" {
		t.Fatalf("server response disclosed private server file path: %q", got)
	}
}

func TestPrivateEvidenceDoesNotRenderForNonReadHTTPMethods(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "known", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2", Status: engine.StatusPassed,
		Runs: []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
	})
	handler := Server{EvidenceRoot: root}.Handler()
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions} {
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(method, "/?run=known", nil))
		if resp.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s status %d, want 405", method, resp.Code)
		}
		if resp.Header().Get("Allow") != "GET, HEAD" {
			t.Errorf("%s missing Allow: %q", method, resp.Header().Get("Allow"))
		}
		if len(resp.Body.String()) > 200 {
			t.Errorf("%s rendered private evidence", method)
		}
		if !strings.Contains(resp.Header().Get("Cache-Control"), "no-store") ||
			resp.Header().Get("X-Frame-Options") != "DENY" ||
			resp.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Errorf("%s lost private evidence response headers", method)
		}
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(method, "/?run=known", nil))
		if resp.Code != http.StatusOK {
			t.Errorf("%s status %d, want 200", method, resp.Code)
		}
	}
}

// Invalid query selectors must fail before loading any local private evidence.
// Otherwise a corrupt/inaccessible evidence root masks malformed links with
// HTTP 500 and wastes disk work on requests that can never be served.
func TestInvalidEvidenceSelectorsAvoidPrivateEvidenceStorageReads(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "synthetic-private-root-file")
	if err := os.WriteFile(root, []byte("fictional broken evidence directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := Server{EvidenceRoot: root}.Handler()
	for _, path := range []string{
		"/?run=%ZZ", "/?run=", "/?source=v1&unknown=something",
		"/?run=one&run=two", "/?source=", "/?bad=123",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("invalid link %q got HTTP %d; want 404 before storage read", path, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "synthetic-private-root-file") ||
			strings.Contains(rec.Body.String(), "fictional broken") {
			t.Errorf("private error exposed for %q", path)
		}
		if rec.Header().Get("Cache-Control") != "no-store, max-age=0" ||
			rec.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Errorf("privacy headers lost: %q", path)
		}
	}
	valid := httptest.NewRecorder()
	handler.ServeHTTP(valid, httptest.NewRequest(http.MethodGet, "/", nil))
	if valid.Code != http.StatusInternalServerError {
		t.Errorf("actual invalid private root should still return 500, got %d", valid.Code)
	}
}
