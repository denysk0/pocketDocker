package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rmAll bool

func init() {
	RmCmd.Flags().BoolVar(&rmAll, "all", false, "remove all container records")
}

var RmCmd = &cobra.Command{
	Use:   "rm [containerID]",
	Short: "Remove container record(s) from store",
	Run: func(cmd *cobra.Command, args []string) {
		var ids []string
		st := getStore()
		if st == nil {
			fmt.Fprintln(os.Stderr, "store not initialized")
			os.Exit(1)
		}

		if rmAll {
			list, err := st.ListContainers()
			if err != nil {
				fmt.Fprintln(os.Stderr, "failed to list containers:", err)
				os.Exit(1)
			}
			for _, c := range list {
				ids = append(ids, c.ID)
			}
			if len(ids) == 0 {
				fmt.Println("No containers to remove")
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
			if err := st.DeleteContainer(id); err != nil {
				fmt.Fprintf(os.Stderr, "failed to remove container %s: %v\n", id, err)
				continue
			}
			fmt.Println("Container", id, "removed from store")
		}
	},
}
