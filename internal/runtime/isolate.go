//go:build linux
// +build linux

package runtime

import (
	"fmt"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"os"
	"syscall"
)

// SetupContainerRoot sets up a new root filesystem using pivot_root
// and mounts /proc and /sys inside the new namespace.
func SetupContainerRoot(rootfsPath string) error {
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("remount / as private failed: %w", err)
	}

	if err := syscall.Mount(rootfsPath, rootfsPath, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mount rootfs failed: %w", err)
	}

	pivotDir := rootfsPath + "/.pivot_root"
	if err := os.MkdirAll(pivotDir, 0700); err != nil {
		return fmt.Errorf("mkdir pivot_root dir failed: %w", err)
	}

	if err := syscall.PivotRoot(rootfsPath, pivotDir); err != nil {
		return fmt.Errorf("pivot_root failed: %w", err)
	}

	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir to / failed: %w", err)
	}

	if err := syscall.Unmount("/.pivot_root", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount old root failed: %w", err)
	}
	if err := os.RemoveAll("/.pivot_root"); err != nil {
		return fmt.Errorf("remove old root dir failed: %w", err)
	}

	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("mount proc failed: %w", err)
	}
	if err := syscall.Mount("sysfs", "/sys", "sysfs", 0, ""); err != nil {
		return fmt.Errorf("mount sysfs failed: %w", err)
	}

	return nil
}

// CloneAndRun clones the current process into new namespaces
// then runs cmdPath with args inside the isolated environment
func CloneAndRun(cmdPath string, args []string, rootfsPath string) (int, *os.File, error) {
	master, slave, err := pty.Open()
	if err != nil {
		return 0, nil, err
	}
	flags := uintptr(syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.SIGCHLD)
	pid, _, errno := syscall.RawSyscall(syscall.SYS_CLONE, flags, 0, 0)
	if errno != 0 {
		master.Close()
		slave.Close()
		return 0, nil, errno
	}
	if pid == 0 {
		master.Close()
		slaveFD := int(slave.Fd())
		unix.Dup2(slaveFD, 1)
		unix.Dup2(slaveFD, 2)
		slave.Close()
		if err := SetupContainerRoot(rootfsPath); err != nil {
			fmt.Fprintf(os.Stderr, "SetupContainerRoot error: %v\n", err)
			os.Exit(1)
		}
		if err := syscall.Exec(cmdPath, append([]string{cmdPath}, args...), os.Environ()); err != nil {
			fmt.Fprintf(os.Stderr, "exec failed: %v\n", err)
			os.Exit(1)
		}
	}
	slave.Close()
	return int(pid), master, nil
}
