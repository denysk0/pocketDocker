package cli

import (
	"fmt"
	"github.com/denysk0/pocketDocker/internal/runtime"
	"github.com/spf13/cobra"
	"os"
	"syscall"
)

var (
	flagInteractive bool
	flagTTY         bool
)

func NewExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec <ID> <cmd> [args...]",
		Short: "run command inside a running container",
		Args:  cobra.MinimumNArgs(2),
		RunE:  execRun,
	}
	cmd.Flags().BoolVarP(&flagInteractive, "interactive", "i", false, "keep stdin open")
	cmd.Flags().BoolVarP(&flagTTY, "tty", "t", false, "allocate a pseudo-TTY")
	return cmd
}

var ExecCmd = NewExecCmd()

func execRun(cmd *cobra.Command, args []string) error {
	id := args[0]
	st := getStore()
	if st == nil {
		return fmt.Errorf("store not initialized")
	}
	info, err := st.GetContainer(id)
	if err != nil {
		return fmt.Errorf("unknown container")
	}
	if info.State != "Running" {
		return fmt.Errorf("container not running")
	}
	// Double-check the PID is still alive; update state if it's gone
	if err := syscall.Kill(info.PID, 0); err == syscall.ESRCH {
		_ = st.UpdateContainerState(id, "Stopped")
		return fmt.Errorf("container not running (PID %d not found)", info.PID)
	}

	cmdArgs := args[1:]
	exitCode, err := runtime.Exec(info.PID, cmdArgs, flagInteractive, flagTTY)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}
