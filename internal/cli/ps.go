package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

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
		if len(list) == 0 {
			fmt.Println("No containers")
			return
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tNAME\tIMAGE\tSTATE\tSTARTED")
		for _, c := range list {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", c.ID, c.Name, c.Image, c.State, c.StartedAt.Format(time.RFC3339))
		}
		tw.Flush()
	},
}
