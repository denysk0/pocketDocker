package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var StopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop container",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("TODO: stop")
		os.Exit(0)
	},
}
