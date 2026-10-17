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
    Status: engine.StatusPassed,
    Runs: []engine.RunResult{{SourceVersion:"v1",TargetVersion:"v2",Status:engine.StatusPassed}},
  })
  marker := "synthetic-private-evidence-id-9841"
  metadata := []byte("{\"schema_version\":1,\"invocation_id\":\""+marker+"\",\"created_at\":\"2026-10-17T10:00:00Z\"}")
  if err := os.WriteFile(filepath.Join(root,"sample","metadata.json"),metadata,0o600);err!=nil{t.Fatal(err)}
  // The internal diagnostic remains actionable for operators and tests.
  if _,err:=LoadBundles(root);err==nil||!strings.Contains(err.Error(),marker){
    t.Fatalf("expected source identity diagnostic, got %v",err)
  }
  response:=httptest.NewRecorder()
  Server{EvidenceRoot:root}.Handler().ServeHTTP(response,httptest.NewRequest(http.MethodGet,"/",nil))
  if response.Code!=http.StatusInternalServerError{t.Fatalf("status %d",response.Code)}
  if got:=response.Body.String();got!="evidence unavailable\n"{
    t.Fatalf("server error disclosed untrusted metadata or path: %q",got)
  }
  if !strings.HasPrefix(response.Header().Get("Content-Type"),"text/plain"){
    t.Fatalf("unexpected evidence failure content type: %q",response.Header().Get("Content-Type"))
  }
}

func TestEvidenceHTTPErrorDoesNotDiscloseLocalFilesystemRoot(t *testing.T) {
  root:=filepath.Join(t.TempDir(),"synthetic-private-evidence-root-1945")
  if err:=os.WriteFile(root,[]byte("not a directory"),0o600);err!=nil{t.Fatal(err)}
  if _,err:=LoadBundles(root);err==nil||!strings.Contains(err.Error(),"synthetic-private-evidence-root-1945"){
    t.Fatalf("expected internal filesystem error with path, got %v",err)
  }
  response:=httptest.NewRecorder()
  Server{EvidenceRoot:root}.Handler().ServeHTTP(response,httptest.NewRequest(http.MethodGet,"/",nil))
  if response.Code!=http.StatusInternalServerError{t.Fatalf("status %d",response.Code)}
  if got:=response.Body.String();got!="evidence unavailable\n"{
    t.Fatalf("server response disclosed private server file path: %q",got)
  }
}
