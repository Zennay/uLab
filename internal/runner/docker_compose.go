package runner

import (
	"context"
	"fmt"

	"github.com/Zennay/ulab/internal/executor"
)

type DockerCompose struct {
	Executor    executor.Executor
	ComposeFile string
	ProjectName string
}

func (r DockerCompose) Prepare(ctx context.Context, env map[string]string) (executor.Result, error) {
	if r.ComposeFile == "" {
		return executor.Result{}, fmt.Errorf("compose file is required")
	}
	return r.Executor.Run(ctx, "docker compose config --quiet", r.environment(env))
}

func (r DockerCompose) Run(ctx context.Context, command string, env map[string]string) (executor.Result, error) {
	return r.Executor.Run(ctx, command, r.environment(env))
}

func (r DockerCompose) Cleanup(ctx context.Context, env map[string]string) (executor.Result, error) {
	return r.Executor.Run(ctx, "docker compose down --volumes --remove-orphans", r.environment(env))
}

func (r DockerCompose) environment(env map[string]string) map[string]string {
	merged := make(map[string]string, len(env)+2)
	for key, value := range env {
		merged[key] = value
	}
	merged["COMPOSE_FILE"] = r.ComposeFile
	if r.ProjectName != "" {
		merged["COMPOSE_PROJECT_NAME"] = r.ProjectName
	}
	return merged
}
