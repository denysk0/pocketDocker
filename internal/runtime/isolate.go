//go:build linux
// +build linux

package runtime

import (
	"fmt"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

type pipePair struct {
	r *os.File
	w *os.File
}

func (p *pipePair) Read(b []byte) (int, error) {
	return p.r.Read(b)
}

func (p *pipePair) Write(b []byte) (int, error) {
	if p.w == nil {
		return 0, io.ErrClosedPipe
	}
	return p.w.Write(b)
}

func (p *pipePair) Close() error {
	if err := p.r.Close(); err != nil {
		return err
	}
	if p.w != nil {
		_ = p.w.Close()
	}
	return nil
}


// SetupContainerRoot sets up a new root filesystem using chroot
// and mounts /proc and /sys inside the new namespace.
func SetupContainerRoot(rootfsPath string) error {
	_ = syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")

	httpdHost := filepath.Join(rootfsPath, "bin", "httpd")
	if _, err := os.Stat(httpdHost); os.IsNotExist(err) {
		_ = os.Symlink("busybox", httpdHost)
	}

	if err := syscall.Chdir(rootfsPath); err != nil {
		return fmt.Errorf("chdir to rootfs failed: %w", err)
	}
	if err := syscall.Chroot("."); err != nil {
		return fmt.Errorf("chroot failed: %w", err)
	}

	procFlags := syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_RELATIME
	if err := syscall.Mount("proc", "/proc", "proc", uintptr(procFlags), ""); err != nil && err != syscall.EPERM {
		return fmt.Errorf("mount proc failed: %w", err)
	}
	sysFlags := syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_RELATIME | syscall.MS_RDONLY
	if err := syscall.Mount("sysfs", "/sys", "sysfs", uintptr(sysFlags), ""); err != nil && err != syscall.EPERM {
		return fmt.Errorf("mount sysfs failed: %w", err)
	}
	return nil
}

// CloneAndRun clones the current process into new namespaces
// then runs cmdPath with args inside the isolated environment
func CloneAndRun(cmdPath string, args []string, rootfsPath string, interactive bool, withTTY bool) (int, io.ReadWriteCloser, error) {
	skipSetup := os.Getenv("SKIP_SETUP") == "1"
	pr, pw, err := os.Pipe()
	if err != nil {
		return 0, nil, err
	}

	var master io.ReadWriteCloser
	var slave *os.File
	var stdinR, stdinW *os.File
	var stdoutR, stdoutW *os.File

	if withTTY {
		masterFile, slaveFile, err := pty.Open()
		if err != nil {
			pr.Close()
			pw.Close()
			return 0, nil, err
		}
		master = masterFile
		slave = slaveFile
	} else {
		stdoutR, stdoutW, err = os.Pipe()
		if err != nil {
			pr.Close()
			pw.Close()
			return 0, nil, err
		}
		if interactive {
			stdinR, stdinW, err = os.Pipe()
			if err != nil {
				stdoutR.Close()
				stdoutW.Close()
				pr.Close()
				pw.Close()
				return 0, nil, err
			}
			master = &pipePair{r: stdoutR, w: stdinW}
		} else {
			master = &pipePair{r: stdoutR}
		}
	}

	flags := uintptr(syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWUSER | syscall.SIGCHLD)
	pid, _, errno := syscall.RawSyscall(syscall.SYS_CLONE, flags, 0, 0)
	if errno != 0 {
		if master != nil {
			master.Close()
		}
		if slave != nil {
			slave.Close()
		}
		if stdoutR != nil {
			stdoutR.Close()
		}
		if stdoutW != nil {
			stdoutW.Close()
		}
		if stdinR != nil {
			stdinR.Close()
		}
		if stdinW != nil {
			stdinW.Close()
		}
		pr.Close()
		pw.Close()
		return 0, nil, errno
	}
	if pid == 0 {
		if master != nil {
			if f, ok := master.(*os.File); ok {
				unix.Close(int(f.Fd()))
			}
		}
		if slave != nil {
			slaveFD := int(slave.Fd())
			unix.Close(int(pw.Fd()))

			buf := make([]byte, 1)
			_, _ = unix.Read(int(pr.Fd()), buf)
			unix.Close(int(pr.Fd()))

			if _, err := unix.Setsid(); err != nil {
				unix.Write(2, []byte(fmt.Sprintf("setsid error: %v\n", err)))
				syscall.Exit(1)
			}
			if err := unix.IoctlSetPointerInt(slaveFD, unix.TIOCSCTTY, 0); err != nil {
				unix.Write(2, []byte(fmt.Sprintf("setctty error: %v\n", err)))
				syscall.Exit(1)
			}
			unix.Dup2(slaveFD, 0)
			unix.Dup2(slaveFD, 1)
			unix.Dup2(slaveFD, 2)
			unix.Close(slaveFD)
		} else {
			unix.Close(int(pw.Fd()))
			buf := make([]byte, 1)
			_, _ = unix.Read(int(pr.Fd()), buf)
			unix.Close(int(pr.Fd()))

			if interactive {
				unix.Dup2(int(stdinR.Fd()), 0)
				unix.Close(int(stdinR.Fd()))
			} else {
				fd, _ := unix.Open("/dev/null", unix.O_RDONLY, 0)
				unix.Dup2(fd, 0)
				unix.Close(fd)
			}
			unix.Dup2(int(stdoutW.Fd()), 1)
			unix.Dup2(int(stdoutW.Fd()), 2)
			unix.Close(int(stdoutW.Fd()))
		}

		if !skipSetup {
			if err := SetupContainerRoot(rootfsPath); err != nil {
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
	
	if err := os.WriteFile(fmt.Sprintf("/proc/%d/setgroups", pid), []byte("deny"), 0644); err != nil {
		if master != nil {
			master.Close()
		}
		if slave != nil {
			slave.Close()
		}
		if stdoutR != nil {
			stdoutR.Close()
		}
		if stdoutW != nil {
			stdoutW.Close()
		}
		if stdinR != nil {
			stdinR.Close()
		}
		if stdinW != nil {
			stdinW.Close()
		}
		pr.Close()
		pw.Close()
		return 0, nil, err
	}
	
	if err := os.WriteFile(fmt.Sprintf("/proc/%d/gid_map", pid), []byte(gidMap), 0644); err != nil {
		if master != nil {
			master.Close()
		}
		if slave != nil {
			slave.Close()
		}
		if stdoutR != nil {
			stdoutR.Close()
		}
		if stdoutW != nil {
			stdoutW.Close()
		}
		if stdinR != nil {
			stdinR.Close()
		}
		if stdinW != nil {
			stdinW.Close()
		}
		pr.Close()
		pw.Close()
		return 0, nil, err
	}
	
	if err := os.WriteFile(fmt.Sprintf("/proc/%d/uid_map", pid), []byte(uidMap), 0644); err != nil {
		if master != nil {
			master.Close()
		}
		if slave != nil {
			slave.Close()
		}
		if stdoutR != nil {
			stdoutR.Close()
		}
		if stdoutW != nil {
			stdoutW.Close()
		}
		if stdinR != nil {
			stdinR.Close()
		}
		if stdinW != nil {
			stdinW.Close()
		}
		pr.Close()
		pw.Close()
		return 0, nil, err
	}

	if withTTY {
		if slave != nil {
			slave.Close()
		}
	} else {
		if stdoutW != nil {
			stdoutW.Close()
		}
		if stdinR != nil {
			stdinR.Close()
		}
	}
	pr.Close()
	pw.Close()

	if interactive {
		return int(pid), master, nil
	}

	if rdr, wtr, errPipe := os.Pipe(); errPipe == nil {
		go func() {
			_, _ = io.Copy(wtr, master)
			master.Close()
			wtr.Close()
		}()
		return int(pid), rdr, nil
	}

	return int(pid), master, nil
}
