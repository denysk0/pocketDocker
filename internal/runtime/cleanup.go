package runtime

import (
	"github.com/denysk0/pocketDocker/internal/runtime/cgroups"
	"github.com/denysk0/pocketDocker/internal/store"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Cleanup stops the container process and removes its resources.
func Cleanup(info store.ContainerInfo) {
	if proc, err := os.FindProcess(info.PID); err == nil {
		if err := proc.Signal(syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		}
		time.Sleep(5 * time.Second)
		if err := proc.Signal(syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		}
	}
	_ = cgroups.RemoveCgroup(info.ID)
	if info.NetworkSetup {
		var pm []PortMap
		if info.Ports != "" {
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
		}
		_ = CleanupNetworkingWithIPSuffix(info.ID, info.IPSuffix, pm, info.IpForwardOrig)
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
	home := os.Getenv("HOME")
	if home == "" {
		if u, err := user.Current(); err == nil {
			home = u.HomeDir
		}
	}
	if home != "" {
		_ = os.Remove(filepath.Join(home, ".pocket-docker", "logs", info.ID+".log"))
	}
}
