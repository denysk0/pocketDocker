package cli

import (
	"fmt"
	"github.com/denysk0/pocketDocker/internal/runtime/cgroups"
	"github.com/spf13/cobra"
	"os"
	"strconv"
	"syscall"
	"time"
)

var StopCmd = &cobra.Command{
	Use:   "stop [containerID]",
	Short: "Stop a container and remove its cgroup",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]

		// Gracefully stop the main container process (assumes id == PID for now)
		if pid, err := strconv.Atoi(id); err == nil {
			if proc, err := os.FindProcess(pid); err == nil {
				_ = proc.Signal(syscall.SIGTERM)
				time.Sleep(5 * time.Second)
				_ = proc.Signal(syscall.SIGKILL)
			}
		}
		// remove cgroup-directory
		if err := cgroups.RemoveCgroup(id); err != nil {
			fmt.Fprintf(os.Stderr, "failed to remove cgroup %s: %v\n", id, err)
			os.Exit(1)
		}

		fmt.Println("Container", id, "stopped and cgroup removed")
	},
}
