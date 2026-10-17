package webui

import "github.com/Zennay/ulab/internal/engine"

// A checksum binds the result bytes, but a passed summary still needs a
// complete, ordered engine lifecycle. The setup hook is optional; upgrade
// and verify are mandatory in every validated configuration. Hook command
// identities must match the frozen config, not just the phase labels.
func hasCoherentPassingPhases(bundle Bundle) bool {
	expected := []engine.Phase{engine.PhasePrepare}
	commands := []string{""}
	if bundle.Config.Setup.Command != "" {
		expected = append(expected, engine.PhaseSetup)
		commands = append(commands, bundle.Config.Setup.Command)
	}
	expected = append(expected, engine.PhaseUpgrade, engine.PhaseVerify, engine.PhaseCleanup)
	commands = append(commands, bundle.Config.Upgrade.Command, bundle.Config.Verify.Command, "")

	for _, run := range bundle.Result.Runs {
		if run.Status != engine.StatusPassed {
			continue
		}
		if run.FailureKind != engine.FailureNone || len(run.Phases) != len(expected) {
			return false
		}
		for i, phase := range run.Phases {
			if phase.Phase != expected[i] || phase.Status != engine.StatusPassed || phase.Command != commands[i] {
				return false
			}
		}
	}
	return true
}
