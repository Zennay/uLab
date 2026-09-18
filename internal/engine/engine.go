package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/Zennay/ulab/internal/executor"
)

type Engine struct {
	Executor executor.Executor
	Now      func() time.Time
}

func (e Engine) Run(ctx context.Context, plan Plan) RunResult {
	now := e.Now
	if now == nil {
		now = time.Now
	}

	started := now()
	result := RunResult{
		SourceVersion: plan.SourceVersion,
		TargetVersion: plan.TargetVersion,
		Status:        StatusPassed,
		StartedAt:     started,
	}

	steps := []struct {
		phase   Phase
		command string
	}{
		{PhaseSetup, plan.SetupCommand},
		{PhaseUpgrade, plan.UpgradeCommand},
		{PhaseVerify, plan.VerifyCommand},
	}

	env := map[string]string{
		"ULAB_SOURCE_VERSION": plan.SourceVersion,
		"ULAB_TARGET_VERSION": plan.TargetVersion,
	}

	for _, step := range steps {
		if step.command == "" {
			continue
		}

		phaseStarted := now()
		phaseResult := PhaseResult{
			Phase:     step.phase,
			Status:    StatusPassed,
			Command:   step.command,
			StartedAt: phaseStarted,
		}

		execResult, err := e.Executor.Run(ctx, step.command, env)
		phaseResult.Duration = now().Sub(phaseStarted)
		phaseResult.Output = execResult.Output
		if err != nil {
			phaseResult.Status = StatusFailed
			phaseResult.Error = fmt.Sprintf("%s phase failed: %v", step.phase, err)
			result.Status = StatusFailed
			result.Phases = append(result.Phases, phaseResult)
			result.Duration = now().Sub(started)
			return result
		}

		result.Phases = append(result.Phases, phaseResult)
	}

	result.Duration = now().Sub(started)
	return result
}
