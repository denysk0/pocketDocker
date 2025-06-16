package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogsCmdPrint(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.Unsetenv("SUDO_USER")
	logDir := filepath.Join(tmp, ".pocket-docker", "logs")
	os.MkdirAll(logDir, 0755)
	path := filepath.Join(logDir, "abc.log")
	content := []byte("hello\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	cmd := NewLogsCmd()
	cmd.SetArgs([]string{"abc"})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), content) {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}

func TestLogsCmdFollow(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.Unsetenv("SUDO_USER")
	logDir := filepath.Join(tmp, ".pocket-docker", "logs")
	os.MkdirAll(logDir, 0755)
	path := filepath.Join(logDir, "abc.log")
	if err := os.WriteFile(path, []byte("first\n"), 0644); err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	cmd := NewLogsCmd()
	cmd.SetArgs([]string{"-f", "abc"})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = cmd.ExecuteContext(ctx)
	}()
	time.Sleep(100 * time.Millisecond)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	f.Write([]byte("second\n"))
	f.Close()
	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)

	out := buf.String()
	if !strings.Contains(out, "first\n") || !strings.Contains(out, "second\n") {
		t.Fatalf("follow output wrong: %q", out)
	}
}
