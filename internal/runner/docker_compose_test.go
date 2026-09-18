package runner

import (
	"context"
	"reflect"
	"testing"

	"github.com/Zennay/ulab/internal/executor"
)

type recordingExecutor struct {
	commands []string
	env      []map[string]string
}

func (e *recordingExecutor) Run(_ context.Context, command string, env map[string]string) (executor.Result, error) {
	e.commands = append(e.commands, command)
	copied := map[string]string{}
	for key, value := range env {
		copied[key] = value
	}
	e.env = append(e.env, copied)
	return executor.Result{}, nil
}

func TestDockerComposeLifecycle(t *testing.T) {
	exec := &recordingExecutor{}
	r := DockerCompose{
		Executor:    exec,
		ComposeFile: "examples/app/compose.yaml",
		ProjectName: "ulab-run-123",
	}

	baseEnv := map[string]string{"ULAB_RUN_ID": "123"}
	if _, err := r.Prepare(context.Background(), baseEnv); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if _, err := r.Run(context.Background(), "./ulab/verify.sh", baseEnv); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := r.Cleanup(context.Background(), baseEnv); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	wantCommands := []string{
		"docker compose config --quiet",
		"./ulab/verify.sh",
		"docker compose down --volumes --remove-orphans",
	}
	if !reflect.DeepEqual(exec.commands, wantCommands) {
		t.Fatalf("commands = %#v, want %#v", exec.commands, wantCommands)
	}
	for _, got := range exec.env {
		if got["COMPOSE_FILE"] != "examples/app/compose.yaml" {
			t.Fatalf("COMPOSE_FILE = %q", got["COMPOSE_FILE"])
		}
		if got["COMPOSE_PROJECT_NAME"] != "ulab-run-123" {
			t.Fatalf("COMPOSE_PROJECT_NAME = %q", got["COMPOSE_PROJECT_NAME"])
		}
	}
}
