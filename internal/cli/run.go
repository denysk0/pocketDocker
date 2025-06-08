package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"github.com/mattn/go-shellwords"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/denysk0/pocketDocker/internal/logging"
	"github.com/denysk0/pocketDocker/internal/runtime"
	"github.com/denysk0/pocketDocker/internal/runtime/cgroups"
	"github.com/denysk0/pocketDocker/internal/store"
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
		var pr io.ReadCloser
		if rootfs == "" || command == "" {
			fmt.Fprintln(os.Stderr, "both --rootfs and --cmd flags are required")
			os.Exit(1)
		}
		// if rootfs is just a name, try lookup in image cache
		if !strings.Contains(rootfs, "/") && !strings.HasSuffix(rootfs, ".tar") {
			if st := getStore(); st != nil {
				if img, err := st.GetImage(rootfs); err == nil {
					rootfs = img.Path
				}
			}
		}
		if _, err := os.Stat(rootfs); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "image or file %s not found (did you run `pull`?)\n", rootfs)
			os.Exit(1)
		}

		if _, err := exec.LookPath("tar"); err != nil {
			fmt.Fprintln(os.Stderr, "tar command not found in PATH")
			os.Exit(1)
		}

		rootfsDir, err := os.MkdirTemp("", "pocketdocker-rootfs-")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		// do not defer os.RemoveAll(rootfsDir)

		idBytes := make([]byte, 16)
		rand.Read(idBytes)
		id := hex.EncodeToString(idBytes)

		fi, _ := os.Stat(rootfs)
		if fi.IsDir() {
			// copy dir using tar pipe to preserve permissions
			cmd1 := exec.Command("tar", "-cC", rootfs, ".")
			cmd2 := exec.Command("tar", "-xC", rootfsDir)
			r, w := io.Pipe()
			cmd1.Stdout = w
			cmd2.Stdin = r
			cmd1.Stderr = os.Stderr
			cmd2.Stderr = os.Stderr
			if err := cmd1.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "copy rootfs: %v\n", err)
				os.Exit(1)
			}
			if err := cmd2.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "copy rootfs: %v\n", err)
				os.Exit(1)
			}
			_ = cmd1.Wait()
			w.Close()
			_ = cmd2.Wait()
		} else {
			tarCmd := exec.Command("tar", "-xf", rootfs, "-C", rootfsDir)
			tarCmd.Stdout = os.Stdout
			tarCmd.Stderr = os.Stderr
			if err := tarCmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "failed to extract rootfs: %v\n", err)
				os.Exit(1)
			}
		}

		parser := shellwords.NewParser()
		parts, err := parser.Parse(command)
		binary := parts[0]
		if !strings.Contains(binary, "/") {
			binary = "/bin/" + binary
		}
		pid, master, err := runtime.CloneAndRun(binary, parts[1:], rootfsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to run command: %v\n", err)
			os.Exit(1)
		}
		if master != nil {
			var errAttach error
			pr, errAttach = logging.Attach(id, master)
			if errAttach != nil {
				fmt.Fprintf(os.Stderr, "failed to attach logs: %v\n", errAttach)
			}
		}

		if memoryLimit > 0 {
			if err := cgroups.ApplyMemoryLimit(id, pid, memoryLimit); err != nil {
				fmt.Fprintf(os.Stderr, "failed to apply memory limit: %v\n", err)
				os.Exit(1)
			}
		}
		if cpuShares > 0 {
			if err := cgroups.ApplyCPUShares(id, pid, cpuShares); err != nil {
				fmt.Fprintf(os.Stderr, "failed to apply CPU shares: %v\n", err)
				os.Exit(1)
			}
		}

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

		// Wait for container process to exit so logs are fully captured
		var ws syscall.WaitStatus
		_, _ = syscall.Wait4(pid, &ws, 0, nil)

		// After exit, close the masterPTY to signal logging.Attach to finish
		if master != nil {
			master.Close()
		}

		// Drain the log pipe to ensure all log data is written
		if pr != nil {
			_, _ = io.Copy(io.Discard, pr)
			pr.Close()
		}

		// Return to allow cobra to exit naturally
		return
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
