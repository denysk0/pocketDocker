package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/denysk0/pocketDocker/internal/util"
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
	home := util.UserHomeDir()
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
	if tailLines != 0 {
		const chunk = 4096
		var (
			totalSize int64
			buf       []byte
			nlCount   int
		)
		if fi, err := f.Stat(); err == nil {
			totalSize = fi.Size()
		}
		offset := int64(0)
		for totalSize+offset > 0 && nlCount <= tailLines {
			step := chunk
			if -offset-int64(step) < 0 {
				step = int(totalSize + offset)
			}
			offset -= int64(step)
			if _, err := f.Seek(offset, io.SeekEnd); err != nil {
				break
			}
			tmp := make([]byte, step)
			n, _ := f.Read(tmp)
			buf = append(tmp[:n], buf...)
			for i := n - 1; i >= 0; i-- {
				if tmp[i] == '\n' {
					nlCount++
					if nlCount > tailLines {
						start := i + 1
						if i > 0 && tmp[i-1] == '\r' {
							start = i
						}
						buf = buf[start:]
						break
					}
				}
			}
		}
		if len(buf) > 0 && !bytes.HasSuffix(buf, []byte("\n")) {
			buf = append(buf, '\n')
		}
		out.Write(buf)
	} else {
		if _, err := f.Seek(0, 0); err == nil {
			io.Copy(out, f)
		}
	}
	ctx := cmd.Context()
	sleepDuration := 50 * time.Millisecond
	maxSleepDuration := 1 * time.Second
	consecutiveEOFs := 0
	
	for {
		buf := make([]byte, 1024)
		n, err := f.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
			sleepDuration = 50 * time.Millisecond
			consecutiveEOFs = 0
		}
		if err != nil {
			if err == io.EOF {
				select {
				case <-ctx.Done():
					return
				default:
				}
				consecutiveEOFs++
				
				if consecutiveEOFs > 3 {
					sleepDuration *= 2
					if sleepDuration > maxSleepDuration {
						sleepDuration = maxSleepDuration
					}
				}
				
				time.Sleep(sleepDuration)
				continue
			}
			fmt.Fprintln(errOut, "read log:", err)
			os.Exit(1)
		}
	}
}

