package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var PsCmd = &cobra.Command{
	Use:   "ps",
	Short: "list containers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("TODO: ps")
		os.Exit(0)
	},
}
