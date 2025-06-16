package runtime

import (
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// ExecRunner interface for running commands with exit code support
type ExecRunner interface {
	RunWithExitCode(cmd string, args ...string) (int, error)
}

// Exec runs a command inside the namespaces of the given PID.
// cmdArgs is the command and its arguments to run (e.g. []string{"ls","-l"}).
// If tty is true, a pseudo-TTY is allocated. When interactive is true,
// the command's stdin is connected to the user's stdin.
// It returns the command exit code.
func Exec(pid int, cmdArgs []string, interactive bool, tty bool) (int, error) {
	return ExecWithRunner(pid, cmdArgs, interactive, tty, nil)
}

// ExecWithRunner is like Exec but allows providing a custom CmdRunner for testing.
// Deprecated: Use ExecWithExecRunner for proper exit code handling.
func ExecWithRunner(pid int, cmdArgs []string, interactive bool, tty bool, r CmdRunner) (int, error) {
	// Put the caller's terminal into raw mode when running an interactive TTY
	var oldState *term.State
	if tty && interactive && term.IsTerminal(int(os.Stdin.Fd())) {
		var err error
		oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
		if err == nil {
			defer term.Restore(int(os.Stdin.Fd()), oldState)
		}
	}

	// Build nsenter arguments dynamically
	pidStr := strconv.Itoa(pid)
	args := []string{"--target", pidStr, "--pid", "--mount", "--uts", "--ipc", "--net"}
	// Add --cgroup if cgroup namespace exists
	if _, err := os.Stat("/proc/" + pidStr + "/ns/cgroup"); err == nil {
		args = append(args, "--cgroup")
	}
	args = append(args, "--")
	args = append(args, cmdArgs...)
	if r != nil {
		if err := r.Run("nsenter", args...); err != nil {
			return -1, err
		}
		return 0, nil
	}

	cmd := exec.Command("nsenter", args...)

	if tty {
		// Allocate a pty pair.
		master, slave, err := pty.Open()
		if err != nil {
			return -1, err
		}
		defer master.Close()
		defer slave.Close()

		// Wire slave to the child.
		cmd.Stdin = slave
		cmd.Stdout = slave
		cmd.Stderr = slave

		// Make the child a session leader and acquire controlling TTY.
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
			Ctty:    0,
		}

		// Start the child.
		if err := cmd.Start(); err != nil {
			return -1, err
		}

		// Put the child’s process group in the foreground of the PTY so that
		// BusyBox job‑control can later restore it without ESRCH.
		pgid := cmd.Process.Pid
		if ioctlErr := unix.IoctlSetInt(int(slave.Fd()), unix.TIOCSPGRP, pgid); ioctlErr != nil {
			// Non‑fatal: continue even if we cannot set foreground pgid.
		}

		// Stream I/O.
		if interactive {
			go io.Copy(master, os.Stdin)
		}
		io.Copy(os.Stdout, master)

		// Wait for child to exit.
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode(), nil
			}
			return -1, err
		}
		return cmd.ProcessState.ExitCode(), nil
	}

	// non‑TTY path
	if interactive {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	if cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode(), nil
	}
	return 0, nil
}

// ExecWithExecRunner is like Exec but allows providing a custom ExecRunner for testing
// with proper exit code handling.
func ExecWithExecRunner(pid int, cmdArgs []string, interactive bool, tty bool, r ExecRunner) (int, error) {
	// Put the caller's terminal into raw mode when running an interactive TTY
	var oldState *term.State
	if tty && interactive && term.IsTerminal(int(os.Stdin.Fd())) {
		var err error
		oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
		if err == nil {
			defer term.Restore(int(os.Stdin.Fd()), oldState)
		}
	}

	// Build nsenter arguments dynamically
	pidStr := strconv.Itoa(pid)
	args := []string{"--target", pidStr, "--pid", "--mount", "--uts", "--ipc", "--net"}
	// Add --cgroup if cgroup namespace exists
	if _, err := os.Stat("/proc/" + pidStr + "/ns/cgroup"); err == nil {
		args = append(args, "--cgroup")
	}
	args = append(args, "--")
	args = append(args, cmdArgs...)
	
	if r != nil {
		// Use the ExecRunner interface that properly returns exit codes
		return r.RunWithExitCode("nsenter", args...)
	}

	cmd := exec.Command("nsenter", args...)

	if tty {
		// Allocate a pty pair.
		master, slave, err := pty.Open()
		if err != nil {
			return -1, err
		}
		defer master.Close()
		defer slave.Close()

		// Wire slave to the child.
		cmd.Stdin = slave
		cmd.Stdout = slave
		cmd.Stderr = slave

		// Make the child a session leader and acquire controlling TTY.
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
			Ctty:    0,
		}

		// Start the child.
		if err := cmd.Start(); err != nil {
			return -1, err
		}

		// Put the child's process group in the foreground of the PTY so that
		// BusyBox job‑control can later restore it without ESRCH.
		pgid := cmd.Process.Pid
		if ioctlErr := unix.IoctlSetInt(int(slave.Fd()), unix.TIOCSPGRP, pgid); ioctlErr != nil {
			// Non‑fatal: continue even if we cannot set foreground pgid.
		}

		// Stream I/O.
		if interactive {
			go io.Copy(master, os.Stdin)
		}
		io.Copy(os.Stdout, master)

		// Wait for child to exit.
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode(), nil
			}
			return -1, err
		}
		return cmd.ProcessState.ExitCode(), nil
	}

	// non‑TTY path
	if interactive {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	if cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode(), nil
	}
	return 0, nil
}
