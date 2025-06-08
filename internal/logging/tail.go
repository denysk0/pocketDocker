package logging

import (
	"bytes"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
)

// Attach reads data from r and writes it to the container log file.
// It returns a reader that receives the same data stream.
func Attach(id string, r io.Reader) (io.ReadCloser, error) {
	home := userHomeDir()
	logDir := filepath.Join(home, ".pocket-docker", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}
	// If we are running as root (e.g. via sudo) ensure the directory is owned by the
	// original user so that non‑sudo commands can read the logs later.
	if os.Geteuid() == 0 {
		if u := sudoUserInfo(); u != nil {
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
		if u := sudoUserInfo(); u != nil {
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
			n, err := r.Read(buf)
			if n > 0 {
				// Strip carriage returns to normalise newlines
				line := bytes.ReplaceAll(buf[:n], []byte{'\r'}, nil)
				if _, werr := f.Write(line); werr == nil {
					pw.Write(line)
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return pr, nil
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

func sudoUserInfo() *user.User {
	sudo := os.Getenv("SUDO_USER")
	if sudo != "" {
		if u, err := user.Lookup(sudo); err == nil {
			return u
		}
	}
	return nil
}

// Append writes a single line to the log file for container id.
func Append(id, line string) {
	home := userHomeDir()
	logDir := filepath.Join(home, ".pocket-docker", "logs")
	os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, id+".log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		f.WriteString(line + "\n")
		f.Close()
	}
}
