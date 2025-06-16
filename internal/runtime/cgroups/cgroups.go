package cgroups

import (
	"bufio"
	"context"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func ensureCgroupDir(containerID string) (string, error) {
	dir := filepath.Join(CgroupRoot, containerID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// CgroupRoot points to the cgroup v2 mount point.
var CgroupRoot = "/sys/fs/cgroup"

// Global map to track OOM monitor cancellation functions and completion
type oomMonitorInfo struct {
	cancel context.CancelFunc
	done   chan struct{}
}

var oomMonitors = make(map[string]*oomMonitorInfo)
var oomMonitorsMutex sync.Mutex

// ApplyMemoryLimit creates a cgroup for containerID and applies the
// provided memory limit in bytes. It also monitors OOM events and
// sends SIGKILL to the process on OOM.
func ApplyMemoryLimit(containerID string, pid int, limitBytes int64) error {
	dir, err := ensureCgroupDir(containerID)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "memory.max"), []byte(strconv.FormatInt(limitBytes, 10)), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0644); err != nil {
		return err
	}

	// Start OOM monitor with cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	oomMonitorsMutex.Lock()
	oomMonitors[containerID] = &oomMonitorInfo{
		cancel: cancel,
		done:   done,
	}
	oomMonitorsMutex.Unlock()
	
	go monitorOOM(ctx, containerID, dir, pid, done)
	return nil
}

// ApplyCPUShares sets CPU weight for containerID cgroup
func ApplyCPUShares(containerID string, pid int, shares int64) error {
	dir, err := ensureCgroupDir(containerID)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "cpu.weight"), []byte(strconv.FormatInt(shares, 10)), 0644); err != nil {
		log.Printf("cpu.weight set failed for %s: %v", containerID, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0644); err != nil {
		return err
	}
	return nil
}

// RemoveCgroup removes cgroup directory for given containerID and stops OOM monitor
func RemoveCgroup(containerID string) error {
	oomMonitorsMutex.Lock()
	if info, exists := oomMonitors[containerID]; exists {
		info.cancel()
		delete(oomMonitors, containerID)
		oomMonitorsMutex.Unlock()
		<-info.done
	} else {
		oomMonitorsMutex.Unlock()
	}
	
	dir := filepath.Join(CgroupRoot, containerID)
	return os.RemoveAll(dir)
}

func monitorOOM(ctx context.Context, containerID, dir string, pid int, done chan struct{}) {
	defer close(done)
	
	f, err := os.Open(filepath.Join(dir, "memory.events"))
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.Seek(0, 0)
			scanner = bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "oom ") || strings.HasPrefix(line, "oom_kill ") {
					fields := strings.Fields(line)
					if len(fields) == 2 && fields[1] != "0" {
						syscall.Kill(pid, syscall.SIGKILL)
						return
					}
				}
			}
		}
	}
}
