package engine

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Zennay/ulab/internal/executor"
)

type fakeExecutor struct {
	commands []string
	failOn   string
	env      []map[string]string
}

func (f *fakeExecutor) Run(_ context.Context, command string, env map[string]string) (executor.Result, error) {
	f.commands = append(f.commands, command)
	copied := map[string]string{}
	for key, value := range env {
		copied[key] = value
	}
	f.env = append(f.env, copied)
	if command == f.failOn {
		return executor.Result{Output: "boom
"}, errors.New("exit 1")
	}
	return executor.Result{Output: command + " ok
"}, nil
}

func TestRunSuccess(t *testing.T) {
	exec := &fakeExecutor{}
	result := Engine{Executor: exec}.Run(context.Background(), Plan{
		SourceVersion:  "v1",
		TargetVersion:  "v2",
		SetupCommand:   "setup",
		UpgradeCommand: "upgrade",
		VerifyCommand:  "verify",
	})

	if result.Status != StatusPassed {
		t.Fatalf("status = %s, want %s", result.Status, StatusPassed)
	}
	want := []string{"setup", "upgrade", "verify"}
	if !reflect.DeepEqual(exec.commands, want) {
		t.Fatalf("commands = %#v, want %#v", exec.commands, want)
	}
	if got := exec.env[0]["ULAB_SOURCE_VERSION"]; got != "v1" {
		t.Fatalf("source env = %q, want v1", got)
	}
	if got := exec.env[0]["ULAB_TARGET_VERSION"]; got != "v2" {
		t.Fatalf("target env = %q, want v2", got)
	}
}

func TestRunStopsAfterFailure(t *testing.T) {
	exec := &fakeExecutor{failOn: "upgrade"}
	result := Engine{Executor: exec}.Run(context.Background(), Plan{
		SourceVersion:  "v1",
		TargetVersion:  "v2",
		SetupCommand:   "setup",
		UpgradeCommand: "upgrade",
		VerifyCommand:  "verify",
	})

	if result.Status != StatusFailed {
		t.Fatalf("status = %s, want %s", result.Status, StatusFailed)
	}
	want := []string{"setup", "upgrade"}
	if !reflect.DeepEqual(exec.commands, want) {
		t.Fatalf("commands = %#v, want %#v", exec.commands, want)
	}
	if len(result.Phases) != 2 {
		t.Fatalf("phases = %d, want 2", len(result.Phases))
	}
	if result.Phases[1].Status != StatusFailed {
		t.Fatalf("upgrade status = %s, want %s", result.Phases[1].Status, StatusFailed)
	}
	if result.Phases[1].Output != "boom
" {
		t.Fatalf("output = %q, want boom", result.Phases[1].Output)
	}
}
