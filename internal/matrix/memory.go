package matrix

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultMemoryReserveBytes = 2 * 1024 * 1024 * 1024
	defaultMemoryPerJobBytes  = 1 * 1024 * 1024 * 1024
	defaultMemoryPollInterval = 2 * time.Second
)

type MemorySnapshot struct {
	AvailableBytes uint64
	TotalBytes     uint64
}

type MemoryReader func() (MemorySnapshot, error)

type MemoryPolicy struct {
	ReserveBytes uint64
	PerJobBytes  uint64
	PollInterval time.Duration
	Read         MemoryReader
}

func DefaultMemoryPolicy() MemoryPolicy {
	return MemoryPolicy{
		ReserveBytes: defaultMemoryReserveBytes,
		PerJobBytes:  defaultMemoryPerJobBytes,
		PollInterval: defaultMemoryPollInterval,
		Read:         readProcMemory,
	}
}

func DefaultAutoJobs() int {
	cpus := runtime.GOMAXPROCS(0)
	if cpus > 1 {
		return cpus - 1
	}
	return 1
}

type memoryGovernor struct {
	maxJobs      int
	reserveBytes uint64
	perJobBytes  uint64
	pollInterval time.Duration
	read         MemoryReader

	mu     sync.Mutex
	active int
}

func newMemoryGovernor(maxJobs int, policy MemoryPolicy) *memoryGovernor {
	defaults := DefaultMemoryPolicy()
	if policy.ReserveBytes == 0 {
		policy.ReserveBytes = defaults.ReserveBytes
	}
	if policy.PerJobBytes == 0 {
		policy.PerJobBytes = defaults.PerJobBytes
	}
	if policy.PollInterval <= 0 {
		policy.PollInterval = defaults.PollInterval
	}
	if policy.Read == nil {
		policy.Read = defaults.Read
	}
	return &memoryGovernor{
		maxJobs:      maxJobs,
		reserveBytes: policy.ReserveBytes,
		perJobBytes:  policy.PerJobBytes,
		pollInterval: policy.PollInterval,
		read:         policy.Read,
	}
}

func (g *memoryGovernor) acquire(ctx context.Context) error {
	for {
		g.mu.Lock()
		if g.active < g.maxJobs {
			allowed := true
			if g.read != nil {
				snapshot, err := g.read()
				if err == nil {
					needed := g.perJobBytes * uint64(g.active+1)
					allowed = snapshot.AvailableBytes > g.reserveBytes &&
						snapshot.AvailableBytes-g.reserveBytes >= needed
				}
			}
			if allowed {
				g.active++
				g.mu.Unlock()
				return nil
			}
		}
		g.mu.Unlock()

		timer := time.NewTimer(g.pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (g *memoryGovernor) release() {
	g.mu.Lock()
	if g.active > 0 {
		g.active--
	}
	g.mu.Unlock()
}

func readProcMemory() (MemorySnapshot, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return MemorySnapshot{}, err
	}

	var snapshot MemorySnapshot
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		unit := uint64(1)
		if len(fields) >= 3 {
			switch strings.ToLower(fields[2]) {
			case "kb":
				unit = 1024
			case "mb":
				unit = 1024 * 1024
			case "gb":
				unit = 1024 * 1024 * 1024
			}
		}
		value *= unit
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemAvailable":
			snapshot.AvailableBytes = value
		case "MemTotal":
			snapshot.TotalBytes = value
		}
	}
	if snapshot.AvailableBytes == 0 {
		return MemorySnapshot{}, errors.New("MemAvailable is missing from /proc/meminfo")
	}
	if snapshot.TotalBytes == 0 {
		return MemorySnapshot{}, fmt.Errorf("MemTotal is missing from /proc/meminfo")
	}
	return snapshot, nil
}
