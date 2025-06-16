#!/usr/bin/env bash
set -euo pipefail

BIN="./pocket-docker"
ROOTFS="./busybox.tar"

# Ensure binary is built
if [ ! -x "$BIN" ]; then
  go build -o pocket-docker ./cmd/pocket-docker
fi

# Cleanup function on exit
cleanup() {
  if [[ -n "${ID-}" ]]; then
    sudo -E "$BIN" stop "$ID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# Start a container that sleeps briefly
ID=$(sudo -E "$BIN" run \
  --rootfs "$ROOTFS" \
  --cmd "/bin/sh -c 'sleep 5'" \
  --restart-max 0)
sleep 1

# Test non-interactive exec output
echo "Testing non-interactive exec..."
OUT=$(sudo -E "$BIN" exec "$ID" -- /bin/sh -c 'echo OK')
if [[ "$OUT" != "OK" ]]; then
  echo "FAIL: Unexpected exec output: '$OUT'"
  exit 1
fi

# Test exit code propagation
echo "Testing exit code propagation..."
set +e
sudo -E "$BIN" exec "$ID" -- /bin/sh -c 'exit 42'
RET=$?
set -e
if [[ $RET -ne 42 ]]; then
  echo "FAIL: Expected exit code 42, got $RET"
  exit 1
fi

echo "All exec tests passed."