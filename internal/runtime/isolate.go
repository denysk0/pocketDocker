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
		// Some kernels (especially with user namespaces) forbid pivot_root for
		// unprivileged users. Fall back to simple chroot in that case.
		if err == syscall.EPERM || err == syscall.EINVAL {
			// Best-effort isolation: chdir & chroot.
			if err2 := syscall.Chdir(rootfsPath); err2 != nil {
				return fmt.Errorf("chdir fallback failed: %w", err2)
			}
			if err2 := syscall.Chroot("."); err2 != nil {
				return fmt.Errorf("chroot fallback failed: %w", err2)
			}
		} else {
			return fmt.Errorf("pivot_root failed: %w", err)
		}
	} else {
		// Normal pivot_root path
		if err := syscall.Chdir("/"); err != nil {
			return fmt.Errorf("chdir to / failed: %w", err)
		}
		if err := syscall.Unmount("/.pivot_root", syscall.MNT_DETACH); err != nil {
			return fmt.Errorf("unmount old root failed: %w", err)
		}
	}
	// NOTE: We intentionally do **not** traverse and delete /.pivot_root here.
	// After MNT_DETACH the old root is already unreachable from any pathname;
	// walking it recursively could take a long time (and even hang) because it
	// would attempt to stat every file in the host’s real root filesystem.
	// Leaving the mount‑point as an empty, detached dir is harmless.

	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		if err != syscall.EPERM {
			return fmt.Errorf("mount proc failed: %w", err)
		}
	}
	if err := syscall.Mount("sysfs", "/sys", "sysfs", 0, ""); err != nil {
		if err != syscall.EPERM {
			return fmt.Errorf("mount sysfs failed: %w", err)
		}
	}

	return nil
}

// CloneAndRun clones the current process into new namespaces
// then runs cmdPath with args inside the isolated environment
func CloneAndRun(cmdPath string, args []string, rootfsPath string) (int, *os.File, *os.File, error) {
	// Synchronization pipe: child waits on read until parent sets up networking
	pr, pw, err := os.Pipe()
	if err != nil {
		return 0, nil, nil, err
	}
	master, slave, err := pty.Open()
	if err != nil {
		return 0, nil, nil, err
	}
	flags := uintptr(syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWUSER | syscall.SIGCHLD)
	pid, _, errno := syscall.RawSyscall(syscall.SYS_CLONE, flags, 0, 0)
	if errno != 0 {
		master.Close()
		slave.Close()
		pr.Close()
		pw.Close()
		return 0, nil, nil, errno
	}
	if pid == 0 {
		master.Close()
		slaveFD := int(slave.Fd())

		// Child: close write end and wait for parent to finish SetupNetworking
		pw.Close()

		// Use stdout for debug before redirecting to PTY
		fmt.Printf("DEBUG: Child process started, PID=%d, waiting for unblock signal\n", os.Getpid())

		buf := make([]byte, 1)
		_, _ = pr.Read(buf)
		pr.Close()

		fmt.Printf("DEBUG: Child process unblocked, continuing\n")

		// Now redirect to PTY
		unix.Dup2(slaveFD, 1)
		unix.Dup2(slaveFD, 2)
		slave.Close()

		// Skip filesystem setup when testing user namespace only
		if os.Getenv("SKIP_SETUP") != "1" {
			fmt.Fprintf(os.Stderr, "DEBUG: Starting SetupContainerRoot...\n")
			if err := SetupContainerRoot(rootfsPath); err != nil {
				fmt.Fprintf(os.Stderr, "SetupContainerRoot error: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "DEBUG: SetupContainerRoot completed\n")
		} else {
			fmt.Fprintf(os.Stderr, "DEBUG: Skipping SetupContainerRoot\n")
		}

		fmt.Fprintf(os.Stderr, "DEBUG: About to exec %s with args %v\n", cmdPath, args)
		if err := syscall.Exec(cmdPath, append([]string{cmdPath}, args...), os.Environ()); err != nil {
			fmt.Fprintf(os.Stderr, "exec failed: %v\n", err)
			os.Exit(1)
		}
	}
	uid := os.Getuid()
	gid := os.Getgid()
	uidMap := fmt.Sprintf("0 %d 1\n", uid)
	gidMap := fmt.Sprintf("0 %d 1\n", gid)
	if err := os.WriteFile(fmt.Sprintf("/proc/%d/uid_map", pid), []byte(uidMap), 0644); err != nil {
		master.Close()
		slave.Close()
		pr.Close()
		pw.Close()
		return 0, nil, nil, err
	}
	_ = os.WriteFile(fmt.Sprintf("/proc/%d/setgroups", pid), []byte("deny"), 0644)
	if err := os.WriteFile(fmt.Sprintf("/proc/%d/gid_map", pid), []byte(gidMap), 0644); err != nil {
		master.Close()
		slave.Close()
		pr.Close()
		pw.Close()
		return 0, nil, nil, err
	}
	slave.Close()
	pr.Close()
	return int(pid), master, pw, nil
}
