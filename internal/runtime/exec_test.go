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
