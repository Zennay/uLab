package webui

import "github.com/Zennay/ulab/internal/engine"

// A stored checksum proves that result bytes were unchanged; it does not
// prove a passed summary is consistent with its recorded phase outcomes.
func hasCoherentPassingPhases(bundle Bundle) bool {
	for _, run := range bundle.Result.Runs {
		if run.Status != engine.StatusPassed {
			continue
		}
		if run.FailureKind != engine.FailureNone {
			return false
		}
		for _, phase := range run.Phases {
			if phase.Status != engine.StatusPassed {
				return false
			}
			switch phase.Phase {
			case engine.PhasePrepare, engine.PhaseSetup, engine.PhaseUpgrade,
				engine.PhaseVerify, engine.PhaseCleanup:
			default:
				return false
			}
		}
	}
	return true
}
