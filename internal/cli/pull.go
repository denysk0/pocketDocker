package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var PullCmd = &cobra.Command{
	Use:   "pull",
	Short: "pull image",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("TODO: pull")
		os.Exit(0)
	},
}
