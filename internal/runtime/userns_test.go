package runtime

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCloneAndRunUserNamespace(t *testing.T) {
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not available")
	}
	rootfs := t.TempDir()
	busyboxTar := filepath.Join("..", "..", "busybox.tar")
	cmd := exec.Command("tar", "-xf", busyboxTar, "-C", rootfs)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("untar busybox: %v: %s", err, out)
	}

	// Check if /bin/sh exists in extracted rootfs
	shPath := filepath.Join(rootfs, "bin", "sh")
	if _, err := os.Stat(shPath); err != nil {
		t.Fatalf("/bin/sh not found in rootfs: %v", err)
	}
	t.Logf("Found /bin/sh at %s", shPath)

	pid, master, err := CloneAndRun("/bin/sh", []string{"-c", "echo starting; id -u; echo done; sleep 0.5"}, rootfs, false, true)
	if err != nil {
		t.Fatalf("CloneAndRun: %v", err)
	}
	defer func() {
		if err := master.Close(); err != nil {
			t.Logf("Error closing master PTY: %v", err)
		}
	}()
	t.Logf("Created process PID=%d", pid)

	// Child process is already unblocked after uid/gid mapping is complete
	t.Logf("Child process started and unblocked")

	// Read output with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan []byte, 1)
	errorCh := make(chan error, 1)

	go func() {
		data, err := io.ReadAll(master)
		if err != nil {
			errorCh <- err
			return
		}
		resultCh <- data
	}()

	// Check process status periodically
	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(1 * time.Second)
			if err := syscall.Kill(pid, 0); err != nil {
				t.Logf("Process %d status check failed after %ds: %v", pid, i+1, err)
				return
			}
			t.Logf("Process %d still alive after %ds", pid, i+1)
		}
	}()

	var outBytes []byte
	select {
	case outBytes = <-resultCh:
		t.Logf("Successfully received data from PTY")
	case err := <-errorCh:
		t.Fatalf("error reading from master: %v", err)
	case <-ctx.Done():
		// Final process check before failing
		if err := syscall.Kill(pid, 0); err != nil {
			t.Fatalf("timeout reading from master PTY - process %d is dead: %v", pid, err)
		} else {
			t.Fatalf("timeout reading from master PTY - process %d still alive but not outputting", pid)
		}
	}

	output := string(outBytes)
	t.Logf("Container output: %q", output)

	// Look for "0" in the output (from id -u command)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var uidLine string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "0" {
			uidLine = line
			break
		}
	}
	if uidLine != "0" {
		t.Fatalf("uid inside container not found or != 0, full output: %q", output)
	}

	hostOut, err := exec.Command("ps", "-o", "uid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		t.Fatalf("ps: %v", err)
	}
	hostUID := strings.TrimSpace(string(hostOut))
	if hostUID != strconv.Itoa(os.Getuid()) {
		t.Fatalf("host uid %s != current uid %d", hostUID, os.Getuid())
	}

	var ws syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &ws, 0, nil); err != nil {
		t.Logf("Wait4 error: %v", err)
	}
}
