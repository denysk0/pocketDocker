//go:build linux
// +build linux

package runtime

import (
	"fmt"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"io" // for io.Copy between PTY and pipe
	"os"
	"path/filepath"
	"syscall"
)

// SetupContainerRoot sets up a new root filesystem using chroot
// and mounts /proc and /sys inside the new namespace.
func SetupContainerRoot(rootfsPath string) error {
	// Make mounts private; ignore errors if unsupported
	_ = syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")

	// Ensure busybox httpd applet is available in rootfs
	httpdHost := filepath.Join(rootfsPath, "bin", "httpd")
	if _, err := os.Stat(httpdHost); os.IsNotExist(err) {
		_ = os.Symlink("/bin/busybox", httpdHost)
	}

	// Enter and chroot into the provided rootfs
	if err := syscall.Chdir(rootfsPath); err != nil {
		return fmt.Errorf("chdir to rootfs failed: %w", err)
	}
	if err := syscall.Chroot("."); err != nil {
		return fmt.Errorf("chroot failed: %w", err)
	}

	// Mount proc filesystem
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil && err != syscall.EPERM {
		return fmt.Errorf("mount proc failed: %w", err)
	}
	// Mount sysfs
	if err := syscall.Mount("sysfs", "/sys", "sysfs", 0, ""); err != nil && err != syscall.EPERM {
		return fmt.Errorf("mount sysfs failed: %w", err)
	}
	return nil
}

// CloneAndRun clones the current process into new namespaces
// then runs cmdPath with args inside the isolated environment
func CloneAndRun(cmdPath string, args []string, rootfsPath string) (int, *os.File, *os.File, error) {
	skipSetup := os.Getenv("SKIP_SETUP") == "1"
	// Synchronization pipe: child waits on read until parent sets up networking
	pr, pw, err := os.Pipe()
	if err != nil {
		return 0, nil, nil, err
	}
	master, slave, err := pty.Open()
	if err != nil {
		pr.Close()
		pw.Close()
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
		masterFD := int(master.Fd())
		unix.Close(masterFD)
		slaveFD := int(slave.Fd())
		// Child: close write end and wait for parent to finish SetupNetworking
		unix.Close(int(pw.Fd()))

		// Wait until the parent either closes the pipe or writes a single byte.
		buf := make([]byte, 1)
		_, _ = unix.Read(int(pr.Fd()), buf)
		unix.Close(int(pr.Fd()))

		// Create new session and set the slave PTY as controlling terminal
		if _, err := unix.Setsid(); err != nil {
			unix.Write(2, []byte(fmt.Sprintf("setsid error: %v\n", err)))
			syscall.Exit(1)
		}
		if err := unix.IoctlSetPointerInt(slaveFD, unix.TIOCSCTTY, 0); err != nil {
			unix.Write(2, []byte(fmt.Sprintf("setctty error: %v\n", err)))
			syscall.Exit(1)
		}

		// Redirect stdin, stdout, and stderr to the slave PTY
		unix.Dup2(slaveFD, 0)
		unix.Dup2(slaveFD, 1)
		unix.Dup2(slaveFD, 2)
		unix.Close(slaveFD)

		// Skip filesystem setup when testing user namespace only
		if !skipSetup {
			if err := SetupContainerRoot(rootfsPath); err != nil {
				// report detailed setup error
				msg := fmt.Sprintf("setup error: %v\n", err)
				unix.Write(2, []byte(msg))
				syscall.Exit(1)
			}
		}

		if err := syscall.Exec(cmdPath, append([]string{cmdPath}, args...), os.Environ()); err != nil {
			unix.Write(2, []byte("exec failed\n"))
			syscall.Exit(1)
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
	// Keep collecting PTY output but present it via a plain pipe so that
	// readers see clean EOF instead of the EIO that Linux returns on PTY
	// masters when the slave side disappears.
	slave.Close()
	pr.Close()

	// Create a pipe and forward all data from the PTY master into it.
	if rdr, wtr, errPipe := os.Pipe(); errPipe == nil {
		go func() {
			_, _ = io.Copy(wtr, master) // copy until PTY closes
			master.Close()              // close original PTY master
			wtr.Close()                 // signal EOF to readers
		}()
		// Return the pipe reader instead of the raw PTY master.
		return int(pid), rdr, pw, nil
	}

	// Fallback: if the pipe could not be created, return the original PTY.
	return int(pid), master, pw, nil
}
