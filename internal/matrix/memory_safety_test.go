package matrix

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// Synthetic telemetry only: never launch a real uLab job or access the VPS.
func safeMemoryTestPolicy(read MemoryReader) MemoryPolicy {
	return MemoryPolicy{ReserveBytes: 1, PerJobBytes: 2, PollInterval: time.Millisecond, Read: read}
}

func TestMemoryAdmissionFailsClosedOnReadError(t *testing.T) {
	var reads atomic.Int32
	g := newMemoryGovernor(2, safeMemoryTestPolicy(func() (MemorySnapshot, error) {
		reads.Add(1)
		return MemorySnapshot{}, errors.New("temporary /proc/meminfo failure")
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	if err := g.acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("missing telemetry admitted new job: %v", err)
	}
	if g.active != 0 || reads.Load() == 0 {
		t.Fatalf("active=%d reads=%d; want no admitted job and a retry", g.active, reads.Load())
	}
}

func TestMemoryAdmissionRecoversOnFreshTelemetry(t *testing.T) {
	var calls atomic.Int32
	g := newMemoryGovernor(1, safeMemoryTestPolicy(func() (MemorySnapshot, error) {
		if calls.Add(1) == 1 {
			return MemorySnapshot{}, errors.New("transient memory read failure")
		}
		return MemorySnapshot{AvailableBytes: 10, TotalBytes: 20}, nil
	}))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := g.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got < 2 {
		t.Fatalf("admitted before telemetry recovered: %d reads", got)
	}
	g.release()
	if g.active != 0 {
		t.Fatalf("release leaked %d active jobs", g.active)
	}
}

func TestMemoryAdmissionRejectsImpossibleTelemetry(t *testing.T) {
	cases := []struct {
		name      string
		available uint64
		total     uint64
	}{
		{name: "more_available_than_total", available: 1000, total: 10},
		{name: "missing_total", available: 1000, total: 0},
		{name: "missing_available", available: 0, total: 1000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newMemoryGovernor(1, safeMemoryTestPolicy(func() (MemorySnapshot, error) {
				return MemorySnapshot{AvailableBytes: tc.available, TotalBytes: tc.total}, nil
			}))
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
			defer cancel()
			if err := g.acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("impossible telemetry admitted work: %v", err)
			}
			if g.active != 0 {
				t.Fatalf("active=%d, want zero", g.active)
			}
		})
	}
}

func TestMemoryAdmissionRejectsUInt64CostOverflow(t *testing.T) {
	g := newMemoryGovernor(2, MemoryPolicy{
		ReserveBytes: 1,
		PerJobBytes: 1 << 63,
		PollInterval: time.Millisecond,
		Read: func() (MemorySnapshot, error) {
			return MemorySnapshot{AvailableBytes: 100, TotalBytes: 100}, nil
		},
	})
	// Old arithmetic: (1<<63) * (active+1) == 0 after uint64 overflow.
	g.active = 1
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	if err := g.acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("overflowing per-job budget admitted work: %v", err)
	}
	if g.active != 1 {
		t.Fatalf("active=%d; an extra job was admitted", g.active)
	}
}

func TestMemoryAdmissionHonorsBoundaries(t *testing.T) {
	var available atomic.Uint64
	available.Store(4)
	g := newMemoryGovernor(2, safeMemoryTestPolicy(func() (MemorySnapshot, error) {
		return MemorySnapshot{AvailableBytes: available.Load(), TotalBytes: 20}, nil
	}))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := g.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if g.active != 1 {
		t.Fatalf("active=%d; expected one job", g.active)
	}
	// Exactly three bytes of budget admit one two-byte slot but not two.
	denied, stop := context.WithTimeout(ctx, 60*time.Millisecond)
	defer stop()
	if err := g.acquire(denied); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second job started without a budget: %v", err)
	}
	available.Store(5) // (5-1) / 2 => two slots
	if err := g.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if g.active != 2 {
		t.Fatalf("active=%d; expected two jobs", g.active)
	}
	g.release()
	g.release()
}

func TestMemoryAdmissionDeniesMissingReaderOrZeroJobCost(t *testing.T) {
	for _, tc := range []string{"missing_reader", "zero_cost"} {
		t.Run(tc, func(t *testing.T) {
			g := newMemoryGovernor(1, safeMemoryTestPolicy(func() (MemorySnapshot, error) {
				return MemorySnapshot{AvailableBytes: 100, TotalBytes: 100}, nil
			}))
			if tc == "missing_reader" {
				g.read = nil
			} else {
				g.perJobBytes = 0
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
			defer cancel()
			if err := g.acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("invalid governor admitted work: %v", err)
			}
		})
	}
}
