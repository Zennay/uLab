package matrix

import (
	"context"
	"sync"

	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/runner"
)

type RunnerFactory func(plan engine.Plan) (runner.Runner, error)

type Result struct {
	TargetVersion string             `json:"target_version"`
	Status        engine.Status      `json:"status"`
	Runs          []engine.RunResult `json:"runs"`
}

type Matrix struct {
	Jobs          int
	RunnerFactory RunnerFactory
}

func (m Matrix) Run(ctx context.Context, plans []engine.Plan) Result {
	result := Result{Status: engine.StatusPassed}
	if len(plans) == 0 {
		return result
	}
	result.TargetVersion = plans[0].TargetVersion
	result.Runs = make([]engine.RunResult, len(plans))

	jobs := m.Jobs
	if jobs < 1 {
		jobs = 1
	}
	if jobs > len(plans) {
		jobs = len(plans)
	}

	indices := make(chan int)
	var wg sync.WaitGroup
	wg.Add(jobs)
	for worker := 0; worker < jobs; worker++ {
		go func() {
			defer wg.Done()
			for index := range indices {
				plan := plans[index]
				if ctx.Err() != nil {
					result.Runs[index] = engine.RunResult{
						SourceVersion: plan.SourceVersion,
						TargetVersion: plan.TargetVersion,
						Status:        engine.StatusCanceled,
						FailureKind:   engine.FailureCanceled,
					}
					continue
				}
				selectedRunner, err := m.RunnerFactory(plan)
				if err != nil {
					result.Runs[index] = engine.RunResult{
						SourceVersion: plan.SourceVersion,
						TargetVersion: plan.TargetVersion,
						Status:        engine.StatusFailed,
						FailureKind:   engine.FailureRunner,
					}
					continue
				}
				result.Runs[index] = engine.Engine{Runner: selectedRunner}.Run(ctx, plan)
			}
		}()
	}

	for index := range plans {
		indices <- index
	}
	close(indices)
	wg.Wait()

	for _, run := range result.Runs {
		if run.Status != engine.StatusPassed {
			result.Status = engine.StatusFailed
			break
		}
	}
	return result
}
