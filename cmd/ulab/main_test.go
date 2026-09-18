package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/matrix"
)

func TestPrintMatrixShowsEveryCompatibilityPath(t *testing.T) {
	result := matrix.Result{
		TargetVersion: "v3.0.0",
		Status:        engine.StatusFailed,
		Runs: []engine.RunResult{
			{SourceVersion: "v1.8.0", TargetVersion: "v3.0.0", Status: engine.StatusPassed},
			{SourceVersion: "v1.9.0", TargetVersion: "v3.0.0", Status: engine.StatusFailed},
			{SourceVersion: "v2.0.0", TargetVersion: "v3.0.0", Status: engine.StatusPassed},
		},
	}

	var out bytes.Buffer
	printMatrix(&out, result)
	text := out.String()
	for _, want := range []string{
		"target v3.0.0: failed",
		"FROM",
		"TARGET",
		"RESULT",
		"v1.8.0",
		"v1.9.0",
		"v2.0.0",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output %q does not contain %q", text, want)
		}
	}
}

func TestRunTestContextPreservesEvidenceWhenJSONOutFails(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "ulab.json")
	evidenceRoot := filepath.Join(root, "evidence")
	badJSONOut := filepath.Join(root, "missing", "result.json")

	config := []byte(`{
  "runner": {"type": "process"},
  "versions": {"from": ["v1"], "to": "v2"},
  "upgrade": {"command": "echo upgrade"},
  "verify": {"command": "echo verify"},
  "policy": {"require_all_paths": true}
}
`)
	if err := os.WriteFile(configPath, config, 0o644); err != nil {
		t.Fatal(err)
	}

	err := runTestContext(context.Background(), []string{
		"--config", configPath,
		"--json-out", badJSONOut,
		"--evidence-root", evidenceRoot,
	})
	if err == nil {
		t.Fatal("expected json output error")
	}
	if !strings.Contains(err.Error(), "write json output") {
		t.Fatalf("error = %q", err)
	}

	entries, readErr := os.ReadDir(evidenceRoot)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		t.Fatalf("evidence entries = %#v", entries)
	}
	for _, name := range []string{"config.json", "result.json", "metadata.json"} {
		if _, statErr := os.Stat(filepath.Join(evidenceRoot, entries[0].Name(), name)); statErr != nil {
			t.Fatalf("missing persisted %s: %v", name, statErr)
		}
	}
}
