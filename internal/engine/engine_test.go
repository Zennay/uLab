package engine

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Zennay/ulab/internal/executor"
)

type fakeRunner struct {
	actions []string
	failOn  string
	env     []map[string]string
}

func (r *fakeRunner) Prepare(_ context.Context, env map[string]string) (executor.Result, error) {
	r.record("prepare", env)
	if r.failOn == "prepare" {
		return executor.Result{Output: "prepare failed\n"}, errors.New("prepare")
	}
	return executor.Result{}, nil
}

func (r *fakeRunner) Run(_ context.Context, command string, env map[string]string) (executor.Result, error) {
	r.record(command, env)
	if r.failOn == command {
		return executor.Result{Output: "boom\n"}, errors.New("exit 1")
	}
	return executor.Result{Output: command + " ok\n"}, nil
}

func (r *fakeRunner) Cleanup(_ context.Context, env map[string]string) (executor.Result, error) {
	r.record("cleanup", env)
	if r.failOn == "cleanup" {
		return executor.Result{Output: "cleanup failed\n"}, errors.New("cleanup")
	}
	return executor.Result{}, nil
}

func (r *fakeRunner) record(action string, env map[string]string) {
	r.actions = append(r.actions, action)
	copied := map[string]string{}
	for key, value := range env {
		copied[key] = value
	}
	r.env = append(r.env, copied)
}

func TestRunSuccess(t *testing.T) {
	r := &fakeRunner{}
	result := Engine{Runner: r, NewID: func() string { return "run-123" }}.Run(context.Background(), Plan{
		SourceVersion:  "v1",
		TargetVersion:  "v2",
		SetupCommand:   "setup",
		UpgradeCommand: "upgrade",
		VerifyCommand:  "verify",
	})

	if result.Status != StatusPassed {
		t.Fatalf("status = %s, want %s", result.Status, StatusPassed)
	}
	want := []string{"prepare", "setup", "upgrade", "verify", "cleanup"}
	if !reflect.DeepEqual(r.actions, want) {
		t.Fatalf("actions = %#v, want %#v", r.actions, want)
	}
	if got := r.env[0]["ULAB_RUN_ID"]; got != "run-123" {
		t.Fatalf("run id env = %q", got)
	}
	if got := r.env[0]["ULAB_SOURCE_VERSION"]; got != "v1" {
		t.Fatalf("source env = %q", got)
	}
	if got := r.env[0]["ULAB_TARGET_VERSION"]; got != "v2" {
		t.Fatalf("target env = %q", got)
	}
}

func TestRunStopsHooksAfterFailureButCleansUp(t *testing.T) {
	r := &fakeRunner{failOn: "upgrade"}
	result := Engine{Runner: r, NewID: func() string { return "run-123" }}.Run(context.Background(), Plan{
		SourceVersion:  "v1",
		TargetVersion:  "v2",
		SetupCommand:   "setup",
		UpgradeCommand: "upgrade",
		VerifyCommand:  "verify",
	})

	if result.Status != StatusFailed {
		t.Fatalf("status = %s, want %s", result.Status, StatusFailed)
	}
	if result.FailureKind != FailureHook {
		t.Fatalf("failure kind = %s", result.FailureKind)
	}
	want := []string{"prepare", "setup", "upgrade", "cleanup"}
	if !reflect.DeepEqual(r.actions, want) {
		t.Fatalf("actions = %#v, want %#v", r.actions, want)
	}
}

func TestCleanupFailureFailsOtherwiseSuccessfulRun(t *testing.T) {
	r := &fakeRunner{failOn: "cleanup"}
	result := Engine{Runner: r, NewID: func() string { return "run-123" }}.Run(context.Background(), Plan{
		SourceVersion:  "v1",
		TargetVersion:  "v2",
		UpgradeCommand: "upgrade",
		VerifyCommand:  "verify",
	})

	if result.Status != StatusFailed {
		t.Fatalf("status = %s, want failed", result.Status)
	}
	if result.FailureKind != FailureRunner {
		t.Fatalf("failure kind = %s, want runner_failed", result.FailureKind)
	}
}
