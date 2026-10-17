package webui

import (
	"testing"

	"github.com/Zennay/ulab/internal/engine"
)

func TestReadyRequiresFrozenHookCommandIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Bundle)
		ready  bool
	}{
		{"legitimate engine hook commands", func(*Bundle) {}, true},
		{"unrecorded upgrade command", func(b *Bundle) { b.Result.Runs[0].Phases[1].Command = "" }, false},
		{"different upgrade command", func(b *Bundle) { b.Result.Runs[0].Phases[1].Command = "./unsafe-upgrade.sh" }, false},
		{"different verify command", func(b *Bundle) { b.Result.Runs[0].Phases[2].Command = "./other-verify.sh" }, false},
		{"swapped upgrade verify commands", func(b *Bundle) {
			b.Result.Runs[0].Phases[1].Command = "./verify.sh"
			b.Result.Runs[0].Phases[2].Command = "./upgrade.sh"
		}, false},
		{"unconfigured lifecycle command", func(b *Bundle) { b.Result.Runs[0].Phases[0].Command = "./pretend.sh" }, false},
		{"cleanup command not engine lifecycle", func(b *Bundle) { b.Result.Runs[0].Phases[3].Command = "./pretend.sh" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			phases := []engine.PhaseResult{
				{Phase: engine.PhasePrepare, Status: engine.StatusPassed},
				{Phase: engine.PhaseUpgrade, Status: engine.StatusPassed, Command: "./upgrade.sh"},
				{Phase: engine.PhaseVerify, Status: engine.StatusPassed, Command: "./verify.sh"},
				{Phase: engine.PhaseCleanup, Status: engine.StatusPassed},
			}
			b := writeCoverageBundle(t, []string{"v1"}, "v2",
				[]engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed, Phases: phases}},
				engine.StatusPassed)
			tc.mutate(&b)
			got := releaseReadiness(b).Label
			if tc.ready && got != "READY" {
				t.Fatalf("valid frozen hook provenance lost READY: %q", got)
			}
			if !tc.ready && got == "READY" {
				t.Fatalf("%s incorrectly authorized release READY", tc.name)
			}
		})
	}
}

func TestSetupHookCommandBoundToConfiguredPlan(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
		ready   bool
	}{
		{"matching configured setup", "./setup.sh", true},
		{"missing configured setup", "", false},
		{"different configured setup", "./other-setup.sh", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := writeCoverageBundle(t, []string{"v1"}, "v2", []engine.RunResult{{
				SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed,
				Phases: []engine.PhaseResult{
					{Phase: engine.PhasePrepare, Status: engine.StatusPassed},
					{Phase: engine.PhaseSetup, Status: engine.StatusPassed, Command: tc.command},
					{Phase: engine.PhaseUpgrade, Status: engine.StatusPassed, Command: "./upgrade.sh"},
					{Phase: engine.PhaseVerify, Status: engine.StatusPassed, Command: "./verify.sh"},
					{Phase: engine.PhaseCleanup, Status: engine.StatusPassed},
				},
			}}, engine.StatusPassed)
			b.Config.Setup.Command = "./setup.sh"
			got := releaseReadiness(b).Label
			if tc.ready && got != "READY" {
				t.Fatalf("valid setup lost READY: %q", got)
			}
			if !tc.ready && got == "READY" {
				t.Fatalf("unbound setup was reported READY: %q", tc.command)
			}
		})
	}
}
