package runtime

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
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

	pid, master, err := CloneAndRun("/bin/sh", []string{"-c", "id -u; sleep 0.5"}, rootfs)
	if err != nil {
		t.Fatalf("CloneAndRun: %v", err)
	}
	defer master.Close()

	outBytes, _ := io.ReadAll(master)
	inside := strings.TrimSpace(string(outBytes))
	if inside != "0" {
		t.Fatalf("uid inside container = %s, want 0", inside)
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
	syscall.Wait4(pid, &ws, 0, nil)
}
