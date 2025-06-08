package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var follow bool
var tailLines int

func NewLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <ID>",
		Short: "view container logs",
		Args:  cobra.ExactArgs(1),
		Run:   logsRun,
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow log output")
	cmd.Flags().IntVar(&tailLines, "tail", 10, "lines to show when following")
	return cmd
}

var LogsCmd = NewLogsCmd()

func logsRun(cmd *cobra.Command, args []string) {
	id := args[0]
	st := getStore()
	if st != nil {
		if _, err := st.GetContainer(id); err != nil {
			fmt.Fprintln(os.Stderr, "unknown container")
			os.Exit(1)
		}
	}
	home := userHomeDir()
	path := filepath.Join(home, ".pocket-docker", "logs", id+".log")
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "unknown container")
		os.Exit(1)
	}
	defer f.Close()
	if !follow {
		// Display the last tailLines lines (or full file if tailLines <= 0)
		data, err := io.ReadAll(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read log:", err)
			os.Exit(1)
		}
		lines := bytes.Split(data, []byte("\n"))
		if tailLines > 0 && len(lines) > tailLines {
			lines = lines[len(lines)-tailLines:]
		}
		for _, line := range lines {
			if len(line) > 0 {
				fmt.Println(string(line))
			}
		}
		return
	}
	data, err := io.ReadAll(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read log:", err)
		os.Exit(1)
	}
	lines := bytes.Split(data, []byte("\n"))
	if tailLines > 0 && len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}
	out := bytes.Join(lines, []byte("\n"))
	if len(out) > 0 {
		if !bytes.HasSuffix(out, []byte("\n")) {
			out = append(out, '\n')
		}
		os.Stdout.Write(out)
	}
	ctx := cmd.Context()
	buf := make([]byte, 1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
		}
		if err != nil {
			if err == io.EOF {
				select {
				case <-ctx.Done():
					return
				case <-time.After(100 * time.Millisecond):
					continue
				}
			}
			fmt.Fprintln(os.Stderr, "read log:", err)
			os.Exit(1)
		}
	}
}

func userHomeDir() string {
	sudo := os.Getenv("SUDO_USER")
	if sudo != "" {
		if u, err := user.Lookup(sudo); err == nil {
			return u.HomeDir
		}
	}
	home, _ := os.UserHomeDir()
	return home
}
