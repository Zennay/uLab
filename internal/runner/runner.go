package runner

import (
	"context"

	"github.com/Zennay/ulab/internal/executor"
)

type Runner interface {
	Prepare(ctx context.Context, env map[string]string) (executor.Result, error)
	Run(ctx context.Context, command string, env map[string]string) (executor.Result, error)
	Cleanup(ctx context.Context, env map[string]string) (executor.Result, error)
}
