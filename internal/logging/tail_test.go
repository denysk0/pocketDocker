package logging

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/creack/pty"
)

func TestAttach(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatalf("openpty: %v", err)
	}
	defer master.Close()
	defer slave.Close()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	logDir := filepath.Join(tmp, ".pocket-docker", "logs")
	os.RemoveAll(logDir)

	r, err := Attach("testid", master)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	defer r.Close()

	msg := "hello\n"
	if _, err := slave.Write([]byte(msg)); err != nil {
		t.Fatalf("write slave: %v", err)
	}

	// allow goroutine to write
	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(r, buf); err != nil {
		t.Fatalf("read from pipe: %v", err)
	}

	logPath := filepath.Join(logDir, "testid.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if string(data) != msg {
		t.Fatalf("log mismatch: %q", string(data))
	}
}
