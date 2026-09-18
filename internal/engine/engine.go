package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Zennay/ulab/internal/executor"
	"github.com/Zennay/ulab/internal/runner"
)

type Engine struct {
	Runner runner.Runner
	Now    func() time.Time
	NewID  func() string
}

func (e Engine) Run(ctx context.Context, plan Plan) RunResult {
	now := e.Now
	if now == nil {
		now = time.Now
	}
	newID := e.NewID
	if newID == nil {
		newID = defaultRunID
	}

	started := now()
	result := RunResult{
		RunID:         newID(),
		SourceVersion: plan.SourceVersion,
		TargetVersion: plan.TargetVersion,
		Status:        StatusPassed,
		StartedAt:     started,
	}
	env := map[string]string{
		"ULAB_RUN_ID":         result.RunID,
		"ULAB_SOURCE_VERSION": plan.SourceVersion,
		"ULAB_TARGET_VERSION": plan.TargetVersion,
	}

	prepare := e.runLifecycle(ctx, now, PhasePrepare, env, e.Runner.Prepare)
	result.Phases = append(result.Phases, prepare)
	if prepare.Status == StatusFailed {
		result.Status = StatusFailed
		result.FailureKind = FailureRunner
	} else {
		steps := []struct {
			phase   Phase
			command string
		}{
			{PhaseSetup, plan.SetupCommand},
			{PhaseUpgrade, plan.UpgradeCommand},
			{PhaseVerify, plan.VerifyCommand},
		}
		for _, step := range steps {
			if step.command == "" {
				continue
			}
			phase := e.runCommand(ctx, now, step.phase, step.command, env)
			result.Phases = append(result.Phases, phase)
			if phase.Status == StatusFailed {
				result.Status = StatusFailed
				result.FailureKind = FailureHook
				break
			}
		}
	}

	cleanup := e.runLifecycle(ctx, now, PhaseCleanup, env, e.Runner.Cleanup)
	result.Phases = append(result.Phases, cleanup)
	if cleanup.Status == StatusFailed && result.Status == StatusPassed {
		result.Status = StatusFailed
		result.FailureKind = FailureRunner
	}

	result.Duration = now().Sub(started)
	return result
}

func (e Engine) runCommand(ctx context.Context, now func() time.Time, phase Phase, command string, env map[string]string) PhaseResult {
	started := now()
	execResult, err := e.Runner.Run(ctx, command, env)
	result := PhaseResult{
		Phase:     phase,
		Status:    StatusPassed,
		Command:   command,
		StartedAt: started,
		Duration:  now().Sub(started),
		Output:    execResult.Output,
	}
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Sprintf("%s phase failed: %v", phase, err)
	}
	return result
}

func (e Engine) runLifecycle(
	ctx context.Context,
	now func() time.Time,
	phase Phase,
	env map[string]string,
	fn func(context.Context, map[string]string) (executor.Result, error),
) PhaseResult {
	started := now()
	execResult, err := fn(ctx, env)
	result := PhaseResult{
		Phase:     phase,
		Status:    StatusPassed,
		StartedAt: started,
		Duration:  now().Sub(started),
		Output:    execResult.Output,
	}
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Sprintf("%s phase failed: %v", phase, err)
	}
	return result
}

func defaultRunID() string {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}
