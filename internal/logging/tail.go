package logging

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/denysk0/pocketDocker/internal/util"
)

// Attach reads data from r and writes it to the container log file.
// It returns a reader that receives the same data stream.
func Attach(id string, r io.Reader) (io.ReadCloser, error) {
	return AttachWithContext(context.Background(), id, r)
}

// AttachWithContext reads data from r and writes it to the container log file.
// It returns a reader that receives the same data stream.
// The goroutine can be cancelled via the provided context.
func AttachWithContext(ctx context.Context, id string, r io.Reader) (io.ReadCloser, error) {
	home := util.UserHomeDir()
	logDir := filepath.Join(home, ".pocket-docker", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}
	// If we are running as root (e.g. via sudo) ensure the directory is owned by the
	// original user so that non‑sudo commands can read the logs later.
	if os.Geteuid() == 0 {
		if u := util.SudoUserInfo(); u != nil {
			uid, _ := strconv.Atoi(u.Uid)
			gid, _ := strconv.Atoi(u.Gid)
			_ = syscall.Chown(logDir, uid, gid)
		}
	}
	logPath := filepath.Join(logDir, id+".log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	if os.Geteuid() == 0 {
		if u := util.SudoUserInfo(); u != nil {
			uid, _ := strconv.Atoi(u.Uid)
			gid, _ := strconv.Atoi(u.Gid)
			_ = syscall.Chown(logPath, uid, gid)
		}
	}
	pr, pw := io.Pipe()
	go func() {
		defer f.Close()
		defer pw.Close()
		buf := make([]byte, 4096)
		
		for {
			// Check if context is cancelled before reading
			select {
			case <-ctx.Done():
				return
			default:
			}
			
			// Blocking read with larger buffer
			n, err := r.Read(buf)
			if n > 0 {
				// Strip carriage returns to normalise newlines
				line := bytes.ReplaceAll(buf[:n], []byte{'\r'}, nil)
				if _, werr := f.Write(line); werr == nil {
					pw.Write(line)
				}
			}
			if err != nil {
				// Error or EOF, exit goroutine
				return
			}
		}
	}()
	return pr, nil
}

// Append writes a single line to the log file for container id.
func Append(id, line string) {
	home := util.UserHomeDir()
	logDir := filepath.Join(home, ".pocket-docker", "logs")
	os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, id+".log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		f.WriteString(line + "\n")
		f.Close()
	}
}
