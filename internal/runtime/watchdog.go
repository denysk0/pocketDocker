package runtime

import (
	"context"
	"os/exec"
	"strconv"
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
				cmd := exec.Command("nsenter", "--target", strconv.Itoa(pid), "--pid", "--mount")
				cmd.Args = append(cmd.Args, "sh", "-c", healthCmd)
				if err := cmd.Run(); err != nil {
					failCh <- struct{}{}
					return
				}
			}
		}
	}()
}
