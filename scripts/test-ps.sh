#!/usr/bin/env bash
set -o errexit -o nounset -o pipefail

export PATH=$PATH:/usr/local/go/bin

BIN=./pocket-docker
ROOTFS=./busybox.tar

echo "1) Build binary"

echo "2) Run a container"
ID=$(sudo $BIN run --rootfs "$ROOTFS" --cmd "/bin/sh -c 'echo hi'" --memory 65536)
echo "Container ID: $ID"

echo "3) PS before stop"
$BIN ps

echo "4) Stop container"
sudo $BIN stop "$ID"

echo "5) PS after stop"
$BIN ps

echo "PS test completed"