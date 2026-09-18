package main

import (
	"bytes"
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
