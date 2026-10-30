package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExplicitRunSelectionDoesNotSilentlyFallBackToEmptyOverview(t *testing.T) {
	handler := Server{EvidenceRoot: t.TempDir()}.Handler()
	for _, tc := range []struct {
		path string
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
