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
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	id := args[0]
	st := getStore()
	if st != nil {
		if _, err := st.GetContainer(id); err != nil {
			fmt.Fprintln(errOut, "unknown container")
			os.Exit(1)
		}
	}
	home := userHomeDir()
	path := filepath.Join(home, ".pocket-docker", "logs", id+".log")
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(errOut, "unknown container")
		os.Exit(1)
	}
	defer f.Close()
	if !follow {
		if _, err := f.Seek(0, 0); err != nil {
			fmt.Fprintln(errOut, "read log:", err)
			os.Exit(1)
		}
		if _, err := io.Copy(out, f); err != nil {
			fmt.Fprintln(errOut, "read log:", err)
			os.Exit(1)
		}
		return
	}
	data, err := io.ReadAll(f)
	if err != nil {
		fmt.Fprintln(errOut, "read log:", err)
		os.Exit(1)
	}
	lines := bytes.Split(data, []byte("\n"))
	if tailLines > 0 && len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}
	outBytes := bytes.Join(lines, []byte("\n"))
	if len(outBytes) > 0 {
		if !bytes.HasSuffix(outBytes, []byte("\n")) {
			outBytes = append(outBytes, '\n')
		}
		out.Write(outBytes)
	}
	ctx := cmd.Context()
	for {
		buf := make([]byte, 1024)
		n, err := f.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
		}
		if err != nil {
			if err == io.EOF {
				select {
				case <-ctx.Done():
					return
				default:
				}
				time.Sleep(50 * time.Millisecond)
				continue
			}
			fmt.Fprintln(errOut, "read log:", err)
			os.Exit(1)
		}
	}
}

func userHomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if sudo := os.Getenv("SUDO_USER"); sudo != "" {
		if u, err := user.Lookup(sudo); err == nil {
			return u.HomeDir
		}
	}
	home, _ := os.UserHomeDir()
	return home
}
