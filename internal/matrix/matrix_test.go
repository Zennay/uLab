package matrix

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

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

type concurrencyProbe struct {
	current int
	max     int
	mu      sync.Mutex
	entered chan struct{}
	release chan struct{}
}

type concurrencyRunner struct {
	probe *concurrencyProbe
}

func (r concurrencyRunner) Prepare(context.Context, map[string]string) (executor.Result, error) {
	return executor.Result{}, nil
}
func (r concurrencyRunner) Run(_ context.Context, command string, _ map[string]string) (executor.Result, error) {
	if command != "upgrade" {
		return executor.Result{}, nil
	}
	r.probe.mu.Lock()
	r.probe.current++
	if r.probe.current > r.probe.max {
		r.probe.max = r.probe.current
	}
	r.probe.mu.Unlock()

	r.probe.entered <- struct{}{}
	<-r.probe.release

	r.probe.mu.Lock()
	r.probe.current--
	r.probe.mu.Unlock()
	return executor.Result{}, nil
}
func (r concurrencyRunner) Cleanup(context.Context, map[string]string) (executor.Result, error) {
	return executor.Result{}, nil
}

func TestMatrixBoundsConcurrency(t *testing.T) {
	probe := &concurrencyProbe{
		entered: make(chan struct{}, 4),
		release: make(chan struct{}),
	}
	plans := []engine.Plan{
		{SourceVersion: "v1", TargetVersion: "v2", UpgradeCommand: "upgrade", VerifyCommand: "verify"},
		{SourceVersion: "v1.1", TargetVersion: "v2", UpgradeCommand: "upgrade", VerifyCommand: "verify"},
		{SourceVersion: "v1.2", TargetVersion: "v2", UpgradeCommand: "upgrade", VerifyCommand: "verify"},
		{SourceVersion: "v1.3", TargetVersion: "v2", UpgradeCommand: "upgrade", VerifyCommand: "verify"},
	}
	m := Matrix{
		Jobs: 2,
		RunnerFactory: func(engine.Plan) (runner.Runner, error) {
			return concurrencyRunner{probe: probe}, nil
		},
	}

	done := make(chan Result, 1)
	go func() {
		done <- m.Run(context.Background(), plans)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-probe.entered:
		case <-time.After(time.Second):
			t.Fatal("expected two upgrade paths to execute concurrently")
		}
	}
	close(probe.release)

	result := <-done
	if result.Status != engine.StatusPassed {
		t.Fatalf("status = %s, want passed", result.Status)
	}

	probe.mu.Lock()
	max := probe.max
	probe.mu.Unlock()
	if max != 2 {
		t.Fatalf("max concurrency = %d, want 2", max)
	}
}
