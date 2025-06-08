package runtime_test

import (
	"context"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/denysk0/pocketDocker/internal/runtime"
)

func TestWatchdogDetectsProcessExit(t *testing.T) {
	// Start a short-lived process
	cmd := exec.Command("sleep", "1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start sleep: %v", err)
	}
	pid := cmd.Process.Pid

	failCh := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Health command empty: watchdog checks pid liveness
	runtime.StartWatchdog(ctx, pid, 500*time.Millisecond, "", failCh)

	// Wait for the process to finish
	if err := cmd.Wait(); err != nil {
		t.Fatalf("sleep exited with error: %v", err)
	}

	select {
	case <-failCh:
		// expected: watchdog detected process exit
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for watchdog to detect process exit")
	}
}

func TestWatchdogDetectsHealthCmdFailure(t *testing.T) {
	// Use current process pid for nsenter (no actual nsenter needed)
	pid := syscall.Getpid()

	failCh := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a health command that always fails
	runtime.StartWatchdog(ctx, pid, 500*time.Millisecond, "/bin/false", failCh)

	select {
	case <-failCh:
		// expected: watchdog detected health check failure
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for watchdog to detect unhealthy container")
	}
}
