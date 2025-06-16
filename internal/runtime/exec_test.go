package runtime

import (
	"reflect"
	"testing"
)

type fakeRunner struct{ cmds [][]string }

func (f *fakeRunner) Run(cmd string, args ...string) error {
	c := append([]string{cmd}, args...)
	f.cmds = append(f.cmds, c)
	return nil
}

type fakeExecRunner struct{ 
	cmds [][]string
	exitCode int
}

func (f *fakeExecRunner) RunWithExitCode(cmd string, args ...string) (int, error) {
	c := append([]string{cmd}, args...)
	f.cmds = append(f.cmds, c)
	return f.exitCode, nil
}

func TestExecBuildsNsenterArgs(t *testing.T) {
	f := &fakeRunner{}
	_, err := ExecWithRunner(5678, []string{"echo", "hi"}, false, false, f)
	if err != nil {
		t.Fatal(err)
	}
	// The expected args slice may have --cgroup, but not --user/--setuid/--setgid.
	// nsenter --target 5678 --pid --mount --uts --ipc --net [--cgroup] -- echo hi
	got := f.cmds[0]
	want := [][]string{
		{"nsenter", "--target", "5678", "--pid", "--mount", "--uts", "--ipc", "--net", "--", "echo", "hi"},
		{"nsenter", "--target", "5678", "--pid", "--mount", "--uts", "--ipc", "--net", "--cgroup", "--", "echo", "hi"},
	}
	match := false
	for _, w := range want {
		if reflect.DeepEqual(got, w) {
			match = true
			break
		}
	}
	if !match {
		t.Fatalf("args mismatch\nwant=%v\n got=%v", want, got)
	}
}

func TestExecWithExecRunnerPreservesExitCode(t *testing.T) {
	testCases := []struct {
		name     string
		exitCode int
	}{
		{"success", 0},
		{"failure", 1},
		{"custom_exit", 42},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeExecRunner{exitCode: tc.exitCode}
			exitCode, err := ExecWithExecRunner(5678, []string{"echo", "hi"}, false, false, f)
			if err != nil {
				t.Fatal(err)
			}
			if exitCode != tc.exitCode {
				t.Fatalf("exit code mismatch: want %d, got %d", tc.exitCode, exitCode)
			}
			// Verify nsenter command was called correctly
			got := f.cmds[0]
			want := [][]string{
				{"nsenter", "--target", "5678", "--pid", "--mount", "--uts", "--ipc", "--net", "--", "echo", "hi"},
				{"nsenter", "--target", "5678", "--pid", "--mount", "--uts", "--ipc", "--net", "--cgroup", "--", "echo", "hi"},
			}
			match := false
			for _, w := range want {
				if reflect.DeepEqual(got, w) {
					match = true
					break
				}
			}
			if !match {
				t.Fatalf("args mismatch\nwant=%v\n got=%v", want, got)
			}
		})
	}
}
