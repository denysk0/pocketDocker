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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/denysk0/pocketDocker/internal/store"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
		if err := cmd1.Wait(); err != nil {
			return "", err
		}
		w.Close()
		if err := cmd2.Wait(); err != nil {
			return "", err
		}
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
	publish        []string
	enableNet      bool
	healthCmd      string
	healthInterval int
	restartMax     int
	detach         bool
	interactive    bool
	tty            bool
)

var RunCmd = &cobra.Command{
	Use:   "run",
	Short: "run a container",
	Run: func(cmd *cobra.Command, args []string) {
		var (
			pr       io.ReadCloser
			oldState *term.State
			rawSet   bool
		)
		defer func() {
			if rawSet {
				_ = term.Restore(int(os.Stdin.Fd()), oldState)
			}
		}()
		if interactive && detach {
			fmt.Fprintln(os.Stderr, "cannot combine --interactive (-i) with --detach (-d)")
			os.Exit(1)
		}
		if tty && detach {
			fmt.Fprintln(os.Stderr, "cannot combine --tty (-t) with --detach (-d)")
			os.Exit(1)
		}
		if rootfs == "" || command == "" {
			fmt.Fprintln(os.Stderr, "both --rootfs and --cmd flags are required")
			os.Exit(1)
		}
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

		name := strings.TrimSuffix(filepath.Base(rootfs), filepath.Ext(rootfs))

		restartCount := 0
		printedID := false
		var ipForwardOrig string
		var ipSuffix int
		for {
			rootfsDir, err := prepareRootfs(rootfs)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			// Resolve the real path of the binary *inside* the extracted rootfs.
			cmdPath := parts[0]
			if !strings.Contains(cmdPath, "/") {
				candidates := []string{
					filepath.Join("/bin", cmdPath),
					filepath.Join("/usr/bin", cmdPath),
					filepath.Join("/usr/local/bin", cmdPath),
				}
				for _, c := range candidates {
					if _, err := os.Stat(filepath.Join(rootfsDir, c)); err == nil {
						cmdPath = c
						break
					}
				}
			}

			if interactive && !rawSet {
				fd := int(os.Stdin.Fd())
				if term.IsTerminal(fd) {
					var err error
					oldState, err = term.MakeRaw(fd)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to set raw mode: %v\n", err)
					} else {
						rawSet = true
					}
				}
			}

			pid, master, err := runtime.CloneAndRun(cmdPath, parts[1:], rootfsDir, interactive, tty)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to run command: %v\n", err)
				os.Exit(1)
			}
			
			ctx, cancel := context.WithCancel(context.Background())
			
			if interactive {
				go func() {
					defer func() { 
						if master != nil {
							master.Close()
						}
					}()
					select {
					case <-ctx.Done():
						return
					default:
						_, _ = io.Copy(master, os.Stdin)
					}
				}()
				go func() { _, _ = io.Copy(os.Stdout, master) }()
			} else {
				if master != nil {
					var errAttach error
					pr, errAttach = logging.Attach(id, master)
					if errAttach != nil {
						fmt.Fprintf(os.Stderr, "failed to attach logs: %v\n", errAttach)
					}
				}
				if pr != nil {
					go func() { _, _ = io.Copy(os.Stdout, pr) }()
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

			if enableNet || len(publish) > 0 {
				var pm []runtime.PortMap
				for _, p := range publish {
					parts := strings.SplitN(p, ":", 2)
					if len(parts) != 2 {
						fmt.Fprintf(os.Stderr, "invalid publish format: %s\n", p)
						os.Exit(1)
					}
					hp, err1 := strconv.Atoi(parts[0])
					cp, err2 := strconv.Atoi(parts[1])
					if err1 != nil || err2 != nil {
						fmt.Fprintf(os.Stderr, "invalid publish format: %s\n", p)
						os.Exit(1)
					}
					pm = append(pm, runtime.PortMap{Host: hp, Container: cp})
				}
				ipForwardOrig, ipSuffix, err = runtime.SetupNetworking(pid, id, pm, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "network setup failed: %v\n", err)
					os.Exit(1)
				}
			} else {
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
				Ports:          strings.Join(publish, ","),
				IpForwardOrig:  ipForwardOrig,
				NetworkSetup:   enableNet || len(publish) > 0,
				IPSuffix:       ipSuffix,
			}
			st := getStore()
			if st != nil {
				_ = st.SaveContainer(info)
			}
			if !printedID {
				fmt.Println(id)
				printedID = true
			}
			if detach {
				go func() {
					var ws syscall.WaitStatus
					syscall.Wait4(pid, &ws, 0, nil)
					
					if st := getStore(); st != nil {
						info.State = "Stopped"
						_ = st.SaveContainer(info)
					}
					runtime.Cleanup(info)
				}()
				return
			}

			if pr != nil && master != nil {
				pr.Close() // Close the old reader
				var errAttach error
				pr, errAttach = logging.AttachWithContext(ctx, id, master)
				if errAttach != nil {
					fmt.Fprintf(os.Stderr, "failed to re-attach logs with context: %v\n", errAttach)
				} else {
					go func() { _, _ = io.Copy(os.Stdout, pr) }()
				}
			}
			
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

			select {
			case <-failCh:
				logging.Append(id, "FAILED health-check")
			case <-exitCh:
			}

			cancel()
			runtime.Cleanup(info)
			if pr != nil {
				_, _ = io.Copy(io.Discard, pr)
				pr.Close()
				pr = nil
			}

			shouldRestart := false
			if restartMax == -1 {
				shouldRestart = true
			} else if restartMax > 0 && restartCount < restartMax {
				shouldRestart = true
			}

			if !shouldRestart {
				if st != nil {
					info.State = "Stopped"
					_ = st.SaveContainer(info)
				}
				cancel()
				return
			}

			restartCount++
			logging.Append(id, fmt.Sprintf("Restart #%d …", restartCount))
		}
		if rawSet {
			term.Restore(int(os.Stdin.Fd()), oldState)
		}
	},
}

func init() {
	RunCmd.Flags().StringVar(&rootfs, "rootfs", "", "path to container rootfs tar")
	RunCmd.Flags().StringVar(&command, "cmd", "", "command to run inside container (e.g. \"/bin/sh\")")
	RunCmd.Flags().Int64Var(&memoryLimit, "memory", 0, "memory limit in bytes (e.g. 104857600 for 100 MB)")
	RunCmd.Flags().Int64Var(&cpuShares, "cpu-shares", 0, "CPU weight 1–10000 (100 = default)")
	RunCmd.Flags().StringArrayVarP(&publish, "publish", "p", nil, "publish port mapping H:C")
	RunCmd.Flags().BoolVar(&enableNet, "network", false, "enable networking namespace")
	RunCmd.Flags().StringVar(&healthCmd, "health-cmd", "", "health check command")
	RunCmd.Flags().IntVar(&healthInterval, "health-interval", 30, "health check interval seconds")
	RunCmd.Flags().IntVar(&restartMax, "restart-max", 0, "max restarts (0 = no restarts, −1 = unlimited)")
	RunCmd.Flags().BoolVarP(&detach, "detach", "d", false, "run container in background")
	RunCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "keep stdin open (forward host STDIN)")
	RunCmd.Flags().BoolVarP(&tty, "tty", "t", false, "allocate a pseudo‑TTY")
	err := RunCmd.MarkFlagRequired("rootfs")
	if err != nil {
		return
	}
	err = RunCmd.MarkFlagRequired("cmd")
	if err != nil {
		return
	}
}
