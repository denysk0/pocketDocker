package cli

import (
	"fmt"
	"os"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// processExists returns true if a PID is present in the kernel’s
// process table.  It treats EPERM (“no permission”) as “exists”.
func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil || err == syscall.EPERM {
		return true
	}
	return false
}

var PsCmd = &cobra.Command{
	Use:   "ps",
	Short: "list containers",
	Run: func(cmd *cobra.Command, args []string) {
		st := getStore()
		if st == nil {
			fmt.Fprintln(os.Stderr, "store not initialized")
			return
		}
		list, err := st.ListContainers()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		// Cleanup stale "Running" states if the kernel no longer knows this PID
		for i, c := range list {
			if c.State == "Running" && !processExists(c.PID) {
				_ = st.UpdateContainerState(c.ID, "Stopped")
				c.State = "Stopped"
				list[i] = c
			}
		}
		if len(list) == 0 {
			fmt.Println("No containers")
			return
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tNAME\tIMAGE\tSTATE\tSTARTED\tRESTARTS")
		for _, c := range list {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\n", c.ID, c.Name, c.Image, c.State, c.StartedAt.Format(time.RFC3339), c.RestartCount)
		}
		tw.Flush()
	},
}
