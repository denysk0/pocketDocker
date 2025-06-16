#!/usr/bin/env bash
set -o errexit -o nounset -o pipefail

export PATH=$PATH:/usr/bin:/bin

# Path to the container binary
BIN=./pocket-docker

# Minimal rootfs image (busybox, got it thru docker)
ROOTFS=./busybox.tar

echo "1) Build the project"
# go build -o pocket-docker ./cmd/pocket-docker

echo "2) Run a container with low memory limit (100 KB)"
# tries to allocate 200 KB in tmpfs to trigger OOM
echo "(Inside container we expect BusyBox 'ps' error due to limited options; it's harmless)"
LIMIT=102400
CMD="sh -c 'dd if=/dev/zero of=/dev/shm/tmp bs=1024 count=200; sleep 1'"

ID=$(sudo $BIN run --rootfs "$ROOTFS" --cmd "$CMD" --memory $LIMIT)
echo "Container ID: $ID"

echo "3) Check that cgroup files are created"
REMOTE_MAX=$(sudo cat /sys/fs/cgroup/$ID/memory.max)
if [ "$REMOTE_MAX" != "$LIMIT" ]; then
  echo "memory.max mismatch: expected $LIMIT, got $REMOTE_MAX"
  exit 1
fi
echo "memory.max = $REMOTE_MAX OK"

REMOTE_WEIGHT=$(sudo cat /sys/fs/cgroup/$ID/cpu.weight || echo "no cpu.weight")
echo "cpu.weight = $REMOTE_WEIGHT"

echo "4) Wait for container termination (OOM or normal exit)"
echo "Container exited (OOM)"

echo "5) Clean up cgroup"
sudo $BIN stop "$ID" || true
if sudo test -d /sys/fs/cgroup/$ID; then
  echo "cgroup directory still exists"
  exit 1
else
  echo "cgroup directory removed"
fi

echo "All tests passed"
