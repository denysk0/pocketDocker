package cli

import (
	"fmt"
	"github.com/denysk0/pocketDocker/internal/runtime"
	"github.com/spf13/cobra"
	"os"
	"strings"
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
		st := getStore()
		if st == nil {
			fmt.Fprintln(os.Stderr, "store not initialized")
			return
		}
		if stopAll {
			list, err := st.ListContainers()
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

		for _, id := range ids {
			id = strings.Trim(strings.TrimSpace(id), "\n\r\t")
			info, err := st.GetContainer(id)
			if err != nil {
				fmt.Fprintln(os.Stderr, "unknown container")
				continue
			}

			runtime.Cleanup(info)

			if err := st.UpdateContainerState(id, "Stopped"); err != nil {
				fmt.Fprintf(os.Stderr, "failed to update container state: %v\n", err)
				continue
			}

			fmt.Println("Container", id, "stopped")
		}
	},
}
