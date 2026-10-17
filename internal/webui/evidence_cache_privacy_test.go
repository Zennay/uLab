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

// The viewer is read-only, but evidence may contain private hook output.
// Caches must not retain successful pages, unknown requests or broken inputs.
func TestEvidenceHTTPResponsesDisablePrivateCaching(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "cache-test", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2",
		Status:        engine.StatusPassed,
		Runs: []engine.RunResult{{
			SourceVersion: "v1",
			TargetVersion: "v2",
			Status:        engine.StatusPassed,
		}},
	})
	server := Server{EvidenceRoot: root}.Handler()
	for _, tc := range []struct {
		path string
		status int
	}{
		{"/", http.StatusOK},
		{"/?run=cache-test", http.StatusOK},
		{"/?run=missing", http.StatusNotFound},
		{"/?source=missing", http.StatusNotFound},
		{"/unknown", http.StatusNotFound},
	} {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("HTTP %d, expected %d", rec.Code, tc.status)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store, max-age=0" {
				t.Fatalf("private evidence can be cached: %q", got)
			}
			if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("missing response MIME sniffing protection: %q", got)
			}
		})
	}
}

func TestEvidenceUnavailableNeverBecomesCacheable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "unreadable-evidence")
	if err := os.WriteFile(root, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("HTTP %d expected generic 500", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store, max-age=0" {
		t.Fatalf("500 can be cached: %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("MIME policy missing for error: %q", got)
	}
	if got := rec.Body.String(); !strings.EqualFold(got, "evidence unavailable\n") {
		t.Fatalf("unexpected error detail: %q", got)
	}
}
