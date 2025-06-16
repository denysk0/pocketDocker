//go:build linux

package runtime

import (
	"context"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// StartWatchdog periodically executes healthCmd (or checks pid liveness if empty).
// If the check fails, it sends a notification on failCh. The watchdog stops when
// the provided context is cancelled.
func StartWatchdog(ctx context.Context, pid int, interval time.Duration, healthCmd string, failCh chan<- struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if healthCmd == "" {
					if err := syscall.Kill(pid, 0); err != nil {
						failCh <- struct{}{}
						return
					}
					continue
				}
				var cmd *exec.Cmd
				needsShell := strings.ContainsAny(healthCmd, "|&;<>()$`\\\"'")
				
				if needsShell {
					cmd = exec.Command("nsenter", "--target", strconv.Itoa(pid),
						"--pid", "--mount", "--uts", "--ipc", "--net",
						"--", "sh", "-c", healthCmd)
				} else {
					parts := strings.Fields(healthCmd)
					if len(parts) == 0 {
						failCh <- struct{}{}
						return
					}
					nsenterArgs := []string{"--target", strconv.Itoa(pid),
						"--pid", "--mount", "--uts", "--ipc", "--net", "--"}
					nsenterArgs = append(nsenterArgs, parts...)
					cmd = exec.Command("nsenter", nsenterArgs...)
				}
				
				out, err := cmd.CombinedOutput()
				if err != nil {
					log.Printf("health check failed: %v, output: %s", err, string(out))
					failCh <- struct{}{}
					return
				}
			}
		}
	}()
}
