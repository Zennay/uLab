package matrix

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/executor"
	"github.com/Zennay/ulab/internal/runner"
)

type fakeRunner struct {
	fail bool
}

func (r fakeRunner) Prepare(context.Context, map[string]string) (executor.Result, error) {
	return executor.Result{}, nil
}
func (r fakeRunner) Run(_ context.Context, _ string, _ map[string]string) (executor.Result, error) {
	if r.fail {
		return executor.Result{}, errors.New("failed")
	}
	return executor.Result{}, nil
}
func (r fakeRunner) Cleanup(context.Context, map[string]string) (executor.Result, error) {
	return executor.Result{}, nil
}

func TestMatrixPreservesPlanOrder(t *testing.T) {
	plans := []engine.Plan{
		{SourceVersion: "v1", TargetVersion: "v2", UpgradeCommand: "upgrade", VerifyCommand: "verify"},
		{SourceVersion: "v1.1", TargetVersion: "v2", UpgradeCommand: "upgrade", VerifyCommand: "verify"},
		{SourceVersion: "v1.2", TargetVersion: "v2", UpgradeCommand: "upgrade", VerifyCommand: "verify"},
	}
	m := Matrix{
		Jobs: 3,
		RunnerFactory: func(plan engine.Plan) (runner.Runner, error) {
			return fakeRunner{}, nil
		},
	}
	result := m.Run(context.Background(), plans)
	got := []string{result.Runs[0].SourceVersion, result.Runs[1].SourceVersion, result.Runs[2].SourceVersion}
	want := []string{"v1", "v1.1", "v1.2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %#v, want %#v", got, want)
	}
	if result.Status != engine.StatusPassed {
		t.Fatalf("status = %s", result.Status)
	}
}

func TestMatrixFailsIfAnyPathFails(t *testing.T) {
	plans := []engine.Plan{
		{SourceVersion: "good", TargetVersion: "v2", UpgradeCommand: "upgrade", VerifyCommand: "verify"},
		{SourceVersion: "bad", TargetVersion: "v2", UpgradeCommand: "upgrade", VerifyCommand: "verify"},
	}
	m := Matrix{
		Jobs: 2,
		RunnerFactory: func(plan engine.Plan) (runner.Runner, error) {
			return fakeRunner{fail: plan.SourceVersion == "bad"}, nil
		},
	}
	result := m.Run(context.Background(), plans)
	if result.Status != engine.StatusFailed {
		t.Fatalf("status = %s, want failed", result.Status)
	}
}
