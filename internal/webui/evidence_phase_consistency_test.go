package webui

import (
	"testing"

	"github.com/Zennay/ulab/internal/engine"
)

// SHA-bound files are not sufficient to certify internally contradictory
// result summaries: the writer permits caller-supplied arbitrary result data.
func TestPassingAggregateCannotHideFailedOrCanceledPhases(t *testing.T) {
	for _, tc := range []struct {
		name  string
		phase engine.Status
	}{
		{"failed verification", engine.StatusFailed},
		{"canceled upgrade", engine.StatusCanceled},
		{"unknown phase status", engine.Status("skipped")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := writeCoverageBundle(t, []string{"v1"}, "v2",
				[]engine.RunResult{{
					SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed,
					Phases: []engine.PhaseResult{
						{Phase: engine.PhasePrepare, Status: engine.StatusPassed},
						{Phase: engine.PhaseUpgrade, Status: tc.phase},
					},
				}}, engine.StatusPassed)
			if got := releaseReadiness(b).Label; got == "READY" {
				t.Fatalf("contradictory %q phase was reported READY", tc.phase)
			}
		})
	}
}

func TestPassingAggregateCannotCarryFailureMarker(t *testing.T) {
	b := writeCoverageBundle(t, []string{"v1"}, "v2",
		[]engine.RunResult{{
			SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed,
			FailureKind: engine.FailureRunner,
			Phases:      []engine.PhaseResult{{Phase: engine.PhaseVerify, Status: engine.StatusPassed}},
		}}, engine.StatusPassed)
	if got := releaseReadiness(b).Label; got == "READY" {
		t.Fatal("run with runner failure marker was reported READY")
	}
}

func TestConsistentPassingPhaseEvidenceRemainsReady(t *testing.T) {
	b := writeCoverageBundle(t, []string{"v1"}, "v2",
		[]engine.RunResult{{
			SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed,
			Phases: []engine.PhaseResult{
				{Phase: engine.PhasePrepare, Status: engine.StatusPassed},
				{Phase: engine.PhaseUpgrade, Status: engine.StatusPassed},
				{Phase: engine.PhaseVerify, Status: engine.StatusPassed},
				{Phase: engine.PhaseCleanup, Status: engine.StatusPassed},
			},
		}}, engine.StatusPassed)
	if got := releaseReadiness(b).Label; got != "READY" {
		t.Fatalf("valid phase evidence lost READY: %q", got)
	}
}
