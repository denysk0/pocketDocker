package cli

import (
	"fmt"
	"github.com/denysk0/pocketDocker/internal/runtime/cgroups"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var stopAll bool

func init() {
	StopCmd.Flags().BoolVar(&stopAll, "all", false, "stop all running containers")
}

var StopCmd = &cobra.Command{
	Use:   "stop [containerID]",
	Short: "Stop a container and remove its cgroup",
	Run: func(cmd *cobra.Command, args []string) {
		var ids []string
		if stopAll {
			list, err := getStore().ListContainers()
			if err != nil {
				fmt.Fprintln(os.Stderr, "failed to list containers:", err)
				os.Exit(1)
			}
			for _, c := range list {
				ids = append(ids, c.ID)
			}
			if len(ids) == 0 {
				fmt.Println("No containers to stop")
				return
			}
		} else {
			if len(args) != 1 {
				fmt.Fprintln(os.Stderr, "requires a container ID or --all")
				os.Exit(1)
			}
			ids = []string{args[0]}
		}

		st := getStore()
		if st == nil {
			fmt.Fprintln(os.Stderr, "store not initialized")
			return
		}
		for _, id := range ids {
			// Trim any whitespace around the ID
			id = strings.TrimSpace(id)
			info, err := st.GetContainer(id)
			if err != nil {
				fmt.Fprintln(os.Stderr, "unknown container")
				continue
			}
			if proc, err := os.FindProcess(info.PID); err == nil {
				_ = proc.Signal(syscall.SIGTERM)
				time.Sleep(5 * time.Second)
				_ = proc.Signal(syscall.SIGKILL)
			}

			if err := cgroups.RemoveCgroup(id); err != nil {
				fmt.Fprintf(os.Stderr, "failed to remove cgroup %s: %v\n", id, err)
				continue
			}

			if err := st.UpdateContainerState(id, "Stopped"); err != nil {
				fmt.Fprintf(os.Stderr, "failed to update container state: %v\n", err)
				continue
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
				// remove the entire directory tree
				if err := os.RemoveAll(info.RootfsDir); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to remove rootfsDir %s: %v\n", info.RootfsDir, err)
				}
			}
			fmt.Println("Container", id, "stopped")
		}
	},
}
