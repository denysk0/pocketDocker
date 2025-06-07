package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/denysk0/pocketDocker/internal/runtime"
	"github.com/spf13/cobra"
)

var (
	rootfs  string
	command string
)

// RunCmd runs a container
var RunCmd = &cobra.Command{
	Use:   "run",
	Short: "run a container",
	Run: func(cmd *cobra.Command, args []string) {
		if rootfs == "" || command == "" {
			fmt.Fprintln(os.Stderr, "both --rootfs and --cmd flags are required")
			os.Exit(1)
		}

		rootfsDir, err := os.MkdirTemp("", "pocketdocker-rootfs-")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer os.RemoveAll(rootfsDir)

		tarCmd := exec.Command("tar", "-xf", rootfs, "-C", rootfsDir)
		tarCmd.Stdout = os.Stdout
		tarCmd.Stderr = os.Stderr
		if err := tarCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to extract rootfs: %v\n", err)
			os.Exit(1)
		}

		parts := strings.Fields(command)
		pid, err := runtime.CloneAndRun(parts[0], parts[1:], rootfsDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		fmt.Println(pid)
		os.Exit(0)
	},
}

func init() {
	RunCmd.Flags().StringVar(&rootfs, "rootfs", "", "path to container rootfs tar")
	RunCmd.Flags().StringVar(&command, "cmd", "", "command to run inside container (e.g. \"/bin/sh\")")
	err := RunCmd.MarkFlagRequired("rootfs")
	if err != nil {
		return
	}
	err = RunCmd.MarkFlagRequired("cmd")
	if err != nil {
		return
	}
}
