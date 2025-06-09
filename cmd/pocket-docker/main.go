package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/denysk0/pocketDocker/internal/cli"
	"github.com/denysk0/pocketDocker/internal/store"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pocket-docker",
	Short: "pocket-docker written in Go",
	Long:  "pocket-docker, commands: run / stop / ps / pull / logs",
}

func main() {
	// if sudo – write DB to home directory of original user
	sudoUser := os.Getenv("SUDO_USER")
	var home string
	if sudoUser != "" {
		u, _ := user.Lookup(sudoUser)
		home = u.HomeDir
	} else {
		home, _ = os.UserHomeDir()
	}
	dbPath := filepath.Join(home, ".pocket-docker", "state.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)
	st, err := store.NewStore(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "store open:", err)
		os.Exit(1)
	}
	if err := st.Init(); err != nil {
		fmt.Fprintln(os.Stderr, "store init:", err)
		os.Exit(1)
	}
	defer func() {
		if err := st.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "store close:", err)
		}
	}()
	cli.SetStore(st)

	rootCmd.AddCommand(cli.RunCmd)
	rootCmd.AddCommand(cli.StopCmd)
	rootCmd.AddCommand(cli.PsCmd)
	rootCmd.AddCommand(cli.PullCmd)
	rootCmd.AddCommand(cli.LogsCmd)
	rootCmd.AddCommand(cli.RmCmd)
	rootCmd.AddCommand(cli.ExecCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
