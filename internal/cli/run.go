package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/denysk0/pocketDocker/internal/logging"
	"github.com/denysk0/pocketDocker/internal/runtime"
	"github.com/denysk0/pocketDocker/internal/runtime/cgroups"
	"github.com/mattn/go-shellwords"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/denysk0/pocketDocker/internal/store"
	"github.com/spf13/cobra"
)

func prepareRootfs(src string) (string, error) {
	dir, err := os.MkdirTemp("", "pocketdocker-rootfs-")
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		cmd1 := exec.Command("tar", "-cC", src, ".")
		cmd2 := exec.Command("tar", "-xC", dir)
		r, w := io.Pipe()
		cmd1.Stdout = w
		cmd2.Stdin = r
		cmd1.Stderr = os.Stderr
		cmd2.Stderr = os.Stderr
		if err := cmd1.Start(); err != nil {
			return "", err
		}
		if err := cmd2.Start(); err != nil {
			return "", err
		}
		_ = cmd1.Wait()
		w.Close()
		_ = cmd2.Wait()
	} else {
		tarCmd := exec.Command("tar", "-xf", src, "-C", dir)
		tarCmd.Stdout = os.Stdout
		tarCmd.Stderr = os.Stderr
		if err := tarCmd.Run(); err != nil {
			return "", err
		}
	}
	return dir, nil
}

var (
	rootfs         string
	command        string
	memoryLimit    int64
	cpuShares      int64
	healthCmd      string
	healthInterval int
	restartMax     int
	detach         bool
)

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

		idBytes := make([]byte, 16)
		rand.Read(idBytes)
		id := hex.EncodeToString(idBytes)

		parser := shellwords.NewParser()
		parts, err := parser.Parse(command)

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		binary := parts[0]
		if !strings.Contains(binary, "/") {
			binary = "/bin/" + binary
		}
		name := strings.TrimSuffix(filepath.Base(rootfs), filepath.Ext(rootfs))

		restartCount := 0
		printedID := false
		for {
			rootfsDir, err := prepareRootfs(rootfs)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
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

			info := store.ContainerInfo{
				ID:             id,
				Name:           name,
				Image:          rootfs,
				PID:            pid,
				State:          "Running",
				StartedAt:      time.Now().UTC(),
				RootfsDir:      rootfsDir,
				RestartCount:   restartCount,
				HealthCmd:      healthCmd,
				HealthInterval: healthInterval,
				RestartMax:     restartMax,
			}
			st := getStore()
			if st != nil {
				_ = st.SaveContainer(info)
			}
			if !printedID {
				fmt.Println(id)
				printedID = true
			}
			// If detached, exit immediately after printing ID
			if detach {
				return
			}

			ctx, cancel := context.WithCancel(context.Background())
			failCh := make(chan struct{}, 1)
			interval := time.Duration(healthInterval) * time.Second
			if interval <= 0 {
				interval = 30 * time.Second
			}
			runtime.StartWatchdog(ctx, pid, interval, healthCmd, failCh)

			exitCh := make(chan struct{})
			go func() {
				var ws syscall.WaitStatus
				syscall.Wait4(pid, &ws, 0, nil)
				close(exitCh)
			}()

			// Wait for either a health-check failure or process exit
			select {
			case <-failCh:
				logging.Append(id, "FAILED health-check")
			case <-exitCh:
				// container exited (normal or error), treat as failure if under restart limit
				if restartMax > 0 && restartCount < restartMax {
					logging.Append(id, "FAILED health-check")
				}
			}

			cancel()
			runtime.Cleanup(info)
			if pr != nil {
				_, _ = io.Copy(io.Discard, pr)
				pr.Close()
				pr = nil
			}

			if restartMax > 0 && restartCount >= restartMax {
				if st != nil {
					info.State = "Stopped"
					st.SaveContainer(info)
				}
				return
			} else if restartMax == 0 {
				// No restarts allowed, exit loop after first run
				if st != nil {
					info.State = "Stopped"
					st.SaveContainer(info)
				}
				return
			}

			restartCount++
			logging.Append(id, fmt.Sprintf("Restart #%d ...", restartCount))
		}
	},
}

func init() {
	RunCmd.Flags().StringVar(&rootfs, "rootfs", "", "path to container rootfs tar")
	RunCmd.Flags().StringVar(&command, "cmd", "", "command to run inside container (e.g. \"/bin/sh\")")
	RunCmd.Flags().Int64Var(&memoryLimit, "memory", 0, "memory limit in bytes (e.g. 104857600 for 100 MB)")
	RunCmd.Flags().Int64Var(&cpuShares, "cpu-shares", 0, "CPU weight 1–10000 (100 = default)")
	RunCmd.Flags().StringVar(&healthCmd, "health-cmd", "", "health check command")
	RunCmd.Flags().IntVar(&healthInterval, "health-interval", 30, "health check interval seconds")
	RunCmd.Flags().IntVar(&restartMax, "restart-max", 0, "max restarts (0 = unlimited)")
	RunCmd.Flags().BoolVarP(&detach, "detach", "d", false, "run container in background")
	err := RunCmd.MarkFlagRequired("rootfs")
	if err != nil {
		return
	}
	err = RunCmd.MarkFlagRequired("cmd")
	if err != nil {
		return
	}
}
