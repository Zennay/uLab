package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Oversized otherwise valid evidence selectors must not perform expensive
// private storage reads; a broken test root proves the ordering deterministically.
func TestOversizedEvidenceLinksFailBeforePrivateStorageReads(t *testing.T) {
	root := filepath.Join(t.TempDir(), "synthetic-private-records")
	if err := os.WriteFile(root, []byte("broken evidence directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := Server{EvidenceRoot: root}.Handler()
	for _, raw := range []string{
		"/?run=" + strings.Repeat("a", 4097),
		"/?source=" + strings.Repeat("v", 4097),
		"/?run=ok&source=" + strings.Repeat("v", 4097),
		"/?run=" + strings.Repeat("a", 2050) + "&source=" + strings.Repeat("b", 2050),
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, raw, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("oversized selector got %d; should be 404 before private storage", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "synthetic-private-records") {
			t.Error("private path disclosed")
		}
		if len(rec.Body.Bytes()) > 200 {
			t.Errorf("response unbounded: %d", len(rec.Body.Bytes()))
		}
		if rec.Header().Get("Cache-Control") != "no-store, max-age=0" {
			t.Error("privacy cache headers absent")
		}
	}
	regular := httptest.NewRecorder()
	handler.ServeHTTP(regular, httptest.NewRequest(http.MethodGet, "/?run=ordinary", nil))
	if regular.Code != http.StatusInternalServerError {
		t.Errorf("ordinary valid query should still expose genuine broken storage as safe 500; got %d", regular.Code)
	}
}
