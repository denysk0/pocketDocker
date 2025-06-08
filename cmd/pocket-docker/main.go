package main

import (
	"fmt"
	"github.com/denysk0/pocketDocker/internal/cli"
	"github.com/spf13/cobra"
	"os"
)

// rootCmd
var rootCmd = &cobra.Command{
	Use:   "pocket-docker",
	Short: "pocket-docker written in Go",
	Long:  "pocket-docker, commands: run / stop / ps / pull / logs",
}

func main() {
	rootCmd.AddCommand(cli.RunCmd)
	rootCmd.AddCommand(cli.StopCmd)
	rootCmd.AddCommand(cli.PsCmd)
	rootCmd.AddCommand(cli.PullCmd)
	rootCmd.AddCommand(cli.LogsCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
