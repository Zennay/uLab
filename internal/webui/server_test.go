package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Zennay/ulab/internal/config"
	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/evidence"
	"github.com/Zennay/ulab/internal/matrix"
)

func TestHandlerRendersBlockedReleaseAndSelectsFirstProblemPath(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)
	writeTestBundle(t, root, "blocked", now, true, matrix.Result{
		TargetVersion: "v3",
		Status:        engine.StatusFailed,
		Runs: []engine.RunResult{
			{
				SourceVersion: "v1",
				TargetVersion: "v3",
				Status:        engine.StatusPassed,
				Duration:      2 * time.Second,
				Phases: []engine.PhaseResult{{
					Phase:    engine.PhaseVerify,
					Status:   engine.StatusPassed,
					Duration: 100 * time.Millisecond,
				}},
			},
			{
				SourceVersion: "v2",
				TargetVersion: "v3",
				Status:        engine.StatusFailed,
				FailureKind:   engine.FailureHook,
				Duration:      3 * time.Second,
				Phases: []engine.PhaseResult{
					{
						Phase:    engine.PhaseUpgrade,
						Status:   engine.StatusPassed,
						Command:  "./upgrade.sh",
						Output:   "migration complete",
						Duration: 2 * time.Second,
					},
					{
						Phase:    engine.PhaseVerify,
						Status:   engine.StatusFailed,
						Command:  "./verify.sh",
						Output:   "checking invariant",
						Error:    "missing invariant",
						Duration: time.Second,
					},
				},
			},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{
		"BLOCKED",
		"1 of 2 tested upgrade paths passed",
		"require_all_paths blocks the release gate",
		"Run timeline",
		"<code>v2</code>",
		"project hook",
		"verify",
		"checking invariant",
		"missing invariant",
		"Reproduce and provenance",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerRendersReadyRelease(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "ready", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2",
		Status:        engine.StatusPassed,
		Runs: []engine.RunResult{{
			SourceVersion: "v1",
			TargetVersion: "v2",
			Status:        engine.StatusPassed,
		}},
	})

	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	body := response.Body.String()
	if !strings.Contains(body, "READY") || !strings.Contains(body, "1 of 1 tested upgrade paths passed") {
		t.Fatalf("ready release summary missing: %s", body)
	}
}

func TestHandlerRendersAttentionWhenPolicyAllowsNonPassingPath(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "attention", time.Now().UTC(), false, matrix.Result{
		TargetVersion: "v2",
		Status:        engine.StatusFailed,
		Runs: []engine.RunResult{{
			SourceVersion: "v1",
			TargetVersion: "v2",
			Status:        engine.StatusFailed,
			FailureKind:   engine.FailureRunner,
		}},
	})

	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	body := response.Body.String()
	for _, want := range []string{"ATTENTION", "policy does not require every path", "runner / infrastructure"} {
		if !strings.Contains(body, want) {
			t.Fatalf("attention summary missing %q: %s", want, body)
		}
	}
}

func TestHandlerSelectsRequestedEvidenceAndSource(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)
	writeTestBundle(t, root, "first", now, true, matrix.Result{
		TargetVersion: "v2",
		Status:        engine.StatusPassed,
		Runs: []engine.RunResult{{
			SourceVersion: "v1",
			TargetVersion: "v2",
			Status:        engine.StatusPassed,
			Phases:        []engine.PhaseResult{{Phase: engine.PhaseVerify, Status: engine.StatusPassed, Output: "first-output"}},
		}},
	})
	writeTestBundle(t, root, "second", now.Add(time.Hour), true, matrix.Result{
		TargetVersion: "v3",
		Status:        engine.StatusPassed,
		Runs: []engine.RunResult{
			{SourceVersion: "v1", TargetVersion: "v3", Status: engine.StatusPassed, Phases: []engine.PhaseResult{{Phase: engine.PhaseVerify, Status: engine.StatusPassed, Output: "v1-output"}}},
			{SourceVersion: "v2", TargetVersion: "v3", Status: engine.StatusPassed, Phases: []engine.PhaseResult{{Phase: engine.PhaseVerify, Status: engine.StatusPassed, Output: "v2-output"}}},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/?run=second&source=v2", nil)
	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	if !strings.Contains(body, "<code>second</code>") || !strings.Contains(body, "v2-output") || strings.Contains(body, "first-output") {
		t.Fatalf("response did not select requested run/source: %s", body)
	}
}

func TestHandlerEscapesEvidenceText(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "safe-html", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2",
		Status:        engine.StatusFailed,
		Runs: []engine.RunResult{{
			SourceVersion: "<script>alert(1)</script>",
			TargetVersion: "v2",
			Status:        engine.StatusFailed,
			FailureKind:   engine.FailureHook,
			Phases: []engine.PhaseResult{{
				Phase:  engine.PhaseVerify,
				Status: engine.StatusFailed,
				Output: "<svg onload=alert(1)>",
				Error:  "<img src=x onerror=alert(1)>",
			}},
		}},
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, request)

	body := response.Body.String()
	for _, raw := range []string{"<script>alert(1)</script>", "<svg onload=alert(1)>", "<img src=x onerror=alert(1)>"} {
		if strings.Contains(body, raw) {
			t.Fatalf("evidence text rendered as raw HTML: %s", body)
		}
	}
	for _, escaped := range []string{"&lt;script&gt;", "&lt;svg", "&lt;img"} {
		if !strings.Contains(body, escaped) {
			t.Fatalf("escaped evidence text %q not visible: %s", escaped, body)
		}
	}
}

func TestHandlerRejectsUnknownSource(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "known", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2",
		Status:        engine.StatusPassed,
		Runs:          []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
	})

	response := httptest.NewRecorder()
	Server{EvidenceRoot: root}.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?source=missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestLoadBundlesRejectsMismatchedInvocationID(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "bundle-id", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2",
		Status:        engine.StatusPassed,
		Runs:          []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
	})

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

func TestLoadBundlesRejectsInvalidConfigSnapshot(t *testing.T) {
	root := t.TempDir()
	writeTestBundle(t, root, "invalid-config", time.Now().UTC(), true, matrix.Result{
		TargetVersion: "v2",
		Status:        engine.StatusPassed,
		Runs:          []engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
	})
	if err := os.WriteFile(filepath.Join(root, "invalid-config", "config.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadBundles(root)
	if err == nil || !strings.Contains(err.Error(), "versions.from") {
		t.Fatalf("LoadBundles error = %v, want invalid config error", err)
	}
}

func writeTestBundle(t *testing.T, root, id string, created time.Time, requireAll bool, result matrix.Result) {
	t.Helper()

	sources := make(config.VersionList, 0, len(result.Runs))
	for _, run := range result.Runs {
		sources = append(sources, run.SourceVersion)
	}
	if len(sources) == 0 {
		sources = config.VersionList{"v1"}
	}
	target := result.TargetVersion
	if target == "" {
		target = "v2"
	}
	cfg := config.Config{
		Runner:   config.Runner{Type: "process"},
		Versions: config.Versions{From: sources, To: target},
		Upgrade:  config.Hook{Command: "./upgrade.sh"},
		Verify:   config.Hook{Command: "./verify.sh"},
		Policy:   config.Policy{RequireAllPaths: requireAll},
	}
	configBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	configBytes = append(configBytes, '\n')

	_, err = evidence.WriteBundle(evidence.BundleInput{
		Root:   root,
		ID:     id,
		Config: configBytes,
		Result: result,
		Metadata: evidence.BundleMetadata{
			CreatedAt:        created,
			ReproduceCommand: []string{"ulab", "test", "--config", "config.json"},
			Jobs:             2,
			SourceVersions:   append([]string(nil), sources...),
			TargetVersion:    target,
			ToolVersion:      "test",
			ToolCommit:       "abc123",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
