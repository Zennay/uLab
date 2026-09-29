package matrix

import (
	"context"
	"testing"
	"time"

	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/executor"
	"github.com/Zennay/ulab/internal/runner"
)

func TestAutoMatrixHonorsAvailableMemory(t *testing.T) {
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
		Jobs: 5,
		Auto: true,
		Memory: MemoryPolicy{
			ReserveBytes: 1 * 1024 * 1024 * 1024,
			PerJobBytes:  1 * 1024 * 1024 * 1024,
			PollInterval: time.Millisecond,
			Read: func() (MemorySnapshot, error) {
				return MemorySnapshot{
					AvailableBytes: 3 * 1024 * 1024 * 1024,
					TotalBytes:     4 * 1024 * 1024 * 1024,
				}, nil
			},
		},
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
	select {
	case <-probe.entered:
		t.Fatal("third path started before memory was released")
	case <-time.After(20 * time.Millisecond):
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
