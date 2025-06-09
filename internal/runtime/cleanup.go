package runtime

import (
	"github.com/denysk0/pocketDocker/internal/runtime/cgroups"
	"github.com/denysk0/pocketDocker/internal/store"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Cleanup stops the container process and removes its resources.
func Cleanup(info store.ContainerInfo) {
	if proc, err := os.FindProcess(info.PID); err == nil {
		_ = proc.Signal(syscall.SIGTERM)
		time.Sleep(5 * time.Second)
		_ = proc.Signal(syscall.SIGKILL)
	}
	_ = cgroups.RemoveCgroup(info.ID)
	// cleanup networking resources
	if info.Ports != "" {
		var pm []PortMap
		for _, p := range strings.Split(info.Ports, ",") {
			parts := strings.SplitN(p, ":", 2)
			if len(parts) == 2 {
				hostPort, err1 := strconv.Atoi(parts[0])
				contPort, err2 := strconv.Atoi(parts[1])
				if err1 == nil && err2 == nil {
					pm = append(pm, PortMap{Host: hostPort, Container: contPort})
				}
			}
		}
		_ = CleanupNetworking(info.ID, pm)
	} else {
		// delete veth even if no ports were published
		_ = CleanupNetworking(info.ID, nil)
	}
	if info.RootfsDir != "" {
		_ = syscall.Unmount(filepath.Join(info.RootfsDir, "proc"), syscall.MNT_DETACH)
		_ = syscall.Unmount(filepath.Join(info.RootfsDir, "sys"), syscall.MNT_DETACH)
		_ = syscall.Unmount(info.RootfsDir, syscall.MNT_DETACH)
		filepath.Walk(info.RootfsDir, func(path string, fi os.FileInfo, err error) error {
			if err == nil {
				os.Chmod(path, 0777)
			}
			return nil
		})
		_ = os.RemoveAll(info.RootfsDir)
	}
}
