package webui

import (
	"testing"

	"github.com/Zennay/ulab/internal/engine"
)

func phaseInventory(phases ...engine.Phase) []engine.PhaseResult {
	results := make([]engine.PhaseResult, len(phases))
	for i, phase := range phases {
		command := ""
		switch phase {
		case engine.PhaseSetup:
			command = "./setup.sh"
		case engine.PhaseUpgrade:
			command = "./upgrade.sh"
		case engine.PhaseVerify:
			command = "./verify.sh"
		}
		results[i] = engine.PhaseResult{Phase: phase, Status: engine.StatusPassed, Command: command}
	}
	return results
}

func TestPassingRunNeedsCompleteOrderedEnginePhaseInventory(t *testing.T) {
	p, u, v, c := engine.PhasePrepare, engine.PhaseUpgrade, engine.PhaseVerify, engine.PhaseCleanup
	for _, tc := range []struct {
		name   string
		phases []engine.PhaseResult
		ready  bool
	}{
		{"empty phases", nil, false},
		{"prepare only", phaseInventory(p), false},
		{"missing prepare", phaseInventory(u, v, c), false},
		{"missing upgrade", phaseInventory(p, v, c), false},
		{"missing verify", phaseInventory(p, u, c), false},
		{"missing cleanup", phaseInventory(p, u, v), false},
		{"duplicate upgrade instead of verify", phaseInventory(p, u, u, c), false},
		{"duplicate verify", phaseInventory(p, u, v, v, c), false},
		{"verify before upgrade", phaseInventory(p, v, u, c), false},
		{"cleanup before verify", phaseInventory(p, u, c, v), false},
		{"unconfigured setup phase", phaseInventory(p, engine.PhaseSetup, u, v, c), false},
		{"valid complete lifecycle", phaseInventory(p, u, v, c), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := writeCoverageBundle(t, []string{"v1"}, "v2", []engine.RunResult{{
				SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed, Phases: tc.phases,
			}}, engine.StatusPassed)
			got := releaseReadiness(b).Label
			if tc.ready && got != "READY" {
				t.Fatalf("complete lifecycle got %q, want READY", got)
			}
			if !tc.ready && got == "READY" {
				t.Fatalf("incomplete/inconsistent phase inventory %q was READY", tc.name)
			}
		})
	}
}

func TestSetupRequiredByFrozenConfigMustBePresentAndOrdered(t *testing.T) {
	p, s, u, v, c := engine.PhasePrepare, engine.PhaseSetup, engine.PhaseUpgrade, engine.PhaseVerify, engine.PhaseCleanup
	for _, tc := range []struct {
		name   string
		phases []engine.PhaseResult
		ready  bool
	}{
		{"configured setup omitted", phaseInventory(p, u, v, c), false},
		{"configured setup late", phaseInventory(p, u, s, v, c), false},
		{"configured setup complete", phaseInventory(p, s, u, v, c), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := writeCoverageBundle(t, []string{"v1"}, "v2", []engine.RunResult{{
				SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed, Phases: tc.phases,
			}}, engine.StatusPassed)
			b.Config.Setup.Command = "./setup.sh"
			got := releaseReadiness(b).Label
			if tc.ready && got != "READY" {
				t.Fatalf("valid configured setup got %q", got)
			}
			if !tc.ready && got == "READY" {
				t.Fatalf("missing/misordered configured setup was READY: %s", tc.name)
			}
		})
	}
}
