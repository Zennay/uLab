package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/matrix"
)

func TestExplicitRunSelectionDoesNotSilentlyFallBackToEmptyOverview(t *testing.T) {
	handler := Server{EvidenceRoot: t.TempDir()}.Handler()
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/", http.StatusOK},
		{"/?run=absent", http.StatusNotFound},
		{"/?source=v1", http.StatusNotFound},
		{"/?run=", http.StatusNotFound},
		{"/?source=", http.StatusNotFound},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Errorf("%s: status %d, want %d", tc.path, rec.Code, tc.status)
		}
	}
}

// An ambiguous shared evidence link must never silently open the first
// matching run/source and display a misleading READY verdict.
func TestEvidenceLinkRejectsDuplicateRunAndSourceParameters(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "known", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2", Status: engine.StatusPassed,
		Runs: []engine.RunResult{{
			SourceVersion: "v1", TargetVersion: "v2",
			Status: engine.StatusPassed,
			Phases: phaseInventory(engine.PhasePrepare, engine.PhaseUpgrade, engine.PhaseVerify, engine.PhaseCleanup),
		}},
	})
	handler := Server{EvidenceRoot: root}.Handler()
	for _, path := range []string{
		"/?run=known&run=unknown",
		"/?run=known&run=known",
		"/?source=v1&source=missing",
		"/?source=v1&source=v1",
		"/?run=known&source=v1&source=",
	} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusNotFound {
				t.Errorf("status %d, want 404 for %q", rec.Code, path)
			}
			if body := rec.Body.String(); body == "" || len(body) > 250 {
				t.Errorf("ambiguous selector returned unexpected evidence content: %q", body)
			}
		})
	}
	for _, path := range []string{"/", "/?run=known", "/?run=known&source=v1"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("unambiguous %q returned %d, want 200", path, rec.Code)
		}
	}
}
