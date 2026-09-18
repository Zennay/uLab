package runner

import (
	"context"

	"github.com/Zennay/ulab/internal/executor"
)

type Process struct {
	Executor executor.Executor
}

func (r Process) Prepare(_ context.Context, _ map[string]string) (executor.Result, error) {
	return executor.Result{}, nil
}

func (r Process) Run(ctx context.Context, command string, env map[string]string) (executor.Result, error) {
	return r.Executor.Run(ctx, command, env)
}

func (r Process) Cleanup(_ context.Context, _ map[string]string) (executor.Result, error) {
	return executor.Result{}, nil
}
