package cgroups

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyMemoryAndCPU(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cgtest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldRoot := CgroupRoot
	CgroupRoot = tmpDir
	defer func() { CgroupRoot = oldRoot }()

	id := "testctn"
	if err := ApplyMemoryLimit(id, 1, 5000); err != nil {
		t.Fatalf("memory limit: %v", err)
	}
	if err := ApplyCPUShares(id, 1, 100); err != nil {
		t.Fatalf("cpu shares: %v", err)
	}

	maxData, err := os.ReadFile(filepath.Join(tmpDir, id, "memory.max"))
	if err != nil {
		t.Fatalf("read memory.max: %v", err)
	}
	if string(maxData) != "5000" {
		t.Fatalf("unexpected memory.max %q", string(maxData))
	}
	weightData, err := os.ReadFile(filepath.Join(tmpDir, id, "cpu.weight"))
	if err != nil {
		t.Fatalf("read cpu.weight: %v", err)
	}
	if string(weightData) != "100" {
		t.Fatalf("unexpected cpu.weight %q", string(weightData))
	}

	if err := RemoveCgroup(id); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, id)); !os.IsNotExist(err) {
		t.Fatalf("cgroup dir still exists")
	}
}
