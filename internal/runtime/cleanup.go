package runtime

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/denysk0/pocketDocker/internal/runtime/cgroups"
	"github.com/denysk0/pocketDocker/internal/store"
)

// Cleanup stops the container process and removes its resources.
func Cleanup(info store.ContainerInfo) {
	if proc, err := os.FindProcess(info.PID); err == nil {
		_ = proc.Signal(syscall.SIGTERM)
		time.Sleep(5 * time.Second)
		_ = proc.Signal(syscall.SIGKILL)
	}
	_ = cgroups.RemoveCgroup(info.ID)
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
