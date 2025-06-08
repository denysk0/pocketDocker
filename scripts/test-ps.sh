#!/usr/bin/env bash
set -o errexit -o nounset -o pipefail

BIN=./pocket-docker
ROOTFS=./busybox.tar

echo "1) Build binary"
go build -o pocket-docker ./cmd/pocket-docker

echo "2) Run a container"
ID=$($BIN run --rootfs "$ROOTFS" --cmd "/bin/sh -c 'echo hi'" --memory 65536)
echo "Container ID: $ID"

echo "3) PS before stop"
$BIN ps

echo "4) Stop container"
$BIN stop "$ID"

echo "5) PS after stop"
$BIN ps