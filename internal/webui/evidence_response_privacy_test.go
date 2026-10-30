package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEvidencePrivateHTMLCannotBeFramedOrLeakReferrer(t *testing.T) {
	h := Server{EvidenceRoot: t.TempDir()}.Handler()
	for _, path := range []string{"/", "/missing", "/?run=absent"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		for name, want := range map[string]string{
			"Content-Security-Policy": "frame-ancestors 'none'",
			"X-Frame-Options":         "DENY",
			"Referrer-Policy":         "no-referrer",
		} {
			if got := rec.Header().Get(name); got != want {
				t.Errorf("%s %s = %q, want %q", path, name, got, want)
			}
		}
	}
}
