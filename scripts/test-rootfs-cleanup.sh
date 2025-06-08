#!/usr/bin/env bash
set -euo pipefail

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "Please install sqlite3 (e.g. sudo apt-get install sqlite3)"
  exit 1
fi

BIN=./pocket-docker
ROOTFS=./busybox.tar
DB="$HOME/.pocket-docker/state.db"



ID=$(sudo $BIN run --rootfs "$ROOTFS" --cmd "/bin/sh")
# allow container to start
sleep 1
DIR=$(sqlite3 "$DB" "select rootfs_dir from containers where id='$ID';")

sudo $BIN stop "$ID"

if [ -d "$DIR" ]; then
  echo "rootfs directory still exists"
  exit 1
else
  echo "rootfs cleanup OK"
fi