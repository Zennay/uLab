//go:build linux

package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProcessCancellationStopsDescendantProcess(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := (Process{}).Run(ctx, fmt.Sprintf("sleep 30 & echo $! > %q; wait", pidFile), nil)
		done <- err
	}()

	var childPID int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			childPID, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			if childPID > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if childPID == 0 {
		t.Fatal("timed out waiting for descendant pid")
	}

	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled process unexpectedly succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled process did not return")
	}

	if processIsRunning(childPID) {
		_ = syscall.Kill(childPID, syscall.SIGKILL)
		t.Fatalf("descendant process %d survived cancellation", childPID)
	}
}

func processIsRunning(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}

	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	fields := strings.Fields(string(data))
	if len(fields) > 2 && fields[2] == "Z" {
		return false
	}
	return true
}
