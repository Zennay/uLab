package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/matrix"
)

func TestExistingEvidenceRejectsExplicitEmptyRunAndSource(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "known", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2", Status: engine.StatusPassed,
		Runs: []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
	})
	handler := Server{EvidenceRoot: root}.Handler()
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/", http.StatusOK},
		{"/?run=", http.StatusNotFound},
		{"/?source=", http.StatusNotFound},
		{"/?run=&source=v1", http.StatusNotFound},
		{"/?run=known&source=", http.StatusNotFound},
		{"/?run=known", http.StatusOK},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != tc.want {
			t.Errorf("%q status %d want %d", tc.path, rec.Code, tc.want)
		}
	}
}
