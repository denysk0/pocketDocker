package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var LogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "view container logs",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("TODO: logs")
		os.Exit(0)
	},
}
