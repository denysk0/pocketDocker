package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/denysk0/pocketDocker/internal/runtime"
	"github.com/denysk0/pocketDocker/internal/runtime/cgroups"
	"github.com/denysk0/pocketDocker/internal/store"
	"github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"
)

var (
	rootfs      string
	command     string
	memoryLimit int64
	cpuShares   int64
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
		// do not defer os.RemoveAll(rootfsDir)

		tarCmd := exec.Command("tar", "-xf", rootfs, "-C", rootfsDir)
		tarCmd.Stdout = os.Stdout
		tarCmd.Stderr = os.Stderr
		if err := tarCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to extract rootfs: %v\n", err)
			os.Exit(1)
		}

		parser := shellwords.NewParser()
		parts, err := parser.Parse(command)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse command: %v\n", err)
			os.Exit(1)
		}
		pid, err := runtime.CloneAndRun(parts[0], parts[1:], rootfsDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if memoryLimit > 0 {
			if err := cgroups.ApplyMemoryLimit(fmt.Sprint(pid), pid, memoryLimit); err != nil {
				fmt.Fprintf(os.Stderr, "failed to apply memory limit: %v\n", err)
				os.Exit(1)
			}
		}
		if cpuShares > 0 {
			if err := cgroups.ApplyCPUShares(fmt.Sprint(pid), pid, cpuShares); err != nil {
				fmt.Fprintf(os.Stderr, "failed to apply CPU shares: %v\n", err)
				os.Exit(1)
			}
		}

		idBytes := make([]byte, 16)
		rand.Read(idBytes)
		id := hex.EncodeToString(idBytes)
		name := strings.TrimSuffix(filepath.Base(rootfs), filepath.Ext(rootfs))
		info := store.ContainerInfo{
			ID:        id,
			Name:      name,
			Image:     rootfs,
			PID:       pid,
			State:     "Running",
			StartedAt: time.Now().UTC(),
			RootfsDir: rootfsDir,
		}
		if st := getStore(); st != nil {
			if err := st.SaveContainer(info); err != nil {
				fmt.Fprintf(os.Stderr, "failed to save container metadata: %v\n", err)
				os.Exit(1)
			}
		}

		fmt.Println(id)
		os.Exit(0)
	},
}

func init() {
	RunCmd.Flags().StringVar(&rootfs, "rootfs", "", "path to container rootfs tar")
	RunCmd.Flags().StringVar(&command, "cmd", "", "command to run inside container (e.g. \"/bin/sh\")")
	RunCmd.Flags().Int64Var(&memoryLimit, "memory", 0, "memory limit in bytes (e.g. 104857600 for 100 MB)")
	RunCmd.Flags().Int64Var(&cpuShares, "cpu-shares", 0, "CPU weight 1–10000 (100 = default)")
	err := RunCmd.MarkFlagRequired("rootfs")
	if err != nil {
		return
	}
	err = RunCmd.MarkFlagRequired("cmd")
	if err != nil {
		return
	}
}
