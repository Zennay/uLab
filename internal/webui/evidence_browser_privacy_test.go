package webui

import (
	"errors"
	"html/template"
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

// A normal successful M5 browser view must not publish the host's evidence
// directory. A browser-only usability participant needn't see local paths.
func TestBrowserEvidenceViewRedactsHostEvidenceRoot(t *testing.T) {
	const secret = "synthetic-local-private-evidence-root-77204"
	root := filepath.Join(t.TempDir(), secret)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestBundle(t, root, "run-safe", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2",
		Status:        engine.StatusPassed,
		Runs: []engine.RunResult{{
			SourceVersion: "v1",
			TargetVersion: "v2",
			Status:        engine.StatusPassed,
		}},
	})
	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(
		response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, marker := range []string{secret, root, "Evidence root:"} {
		if strings.Contains(body, marker) {
			t.Fatalf("browser leaked evidence directory marker %q", marker)
		}
	}
	if !strings.Contains(body, "Read-only view") || !strings.Contains(body, "run-safe") {
		t.Fatal("redaction removed essential evidence navigation")
	}
}

// A template can emit bytes and then return an error. This must be a clean
// generic 500, never a partial HTML 200 or a reflected error detail.
func TestBrowserRenderFailureDropsPartialEvidenceAndInternalError(t *testing.T) {
	const privateMarker = "synthetic-private-hook-output-11358"
	broken := template.Must(template.New("synthetic").Funcs(
		template.FuncMap{"fail": func() (string, error) {
			return "", errors.New(privateMarker)
		}},
	).Parse("PARTIAL-PRIVATE-PREFIX{{fail}}"))
	response := httptest.NewRecorder()
	writeEvidencePage(response, broken, pageData{})
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d: expected fail-closed HTTP 500", response.Code)
	}
	if got := response.Body.String(); got != "evidence unavailable\n" {
		t.Fatalf("render leaked a partial page or internal error: %q", got)
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("unexpected private render error content type: %q", got)
	}
}

func TestSuccessfulBufferedBrowserRenderPreservesHTMLContentType(t *testing.T) {
	view := template.Must(template.New("ok").Parse("<h1>Local evidence</h1>"))
	response := httptest.NewRecorder()
	writeEvidencePage(response, view, pageData{})
	if response.Code != http.StatusOK || response.Body.String() != "<h1>Local evidence</h1>" {
		t.Fatalf("successful render changed: status=%d body=%q", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("HTML content type changed: %q", got)
	}
}
