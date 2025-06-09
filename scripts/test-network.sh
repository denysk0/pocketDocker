#!/usr/bin/env bash

set -euo pipefail

# Cleanup stale veth interfaces and NAT rules
for iface in $(ip -o link show | awk -F': ' '/veth/ {print $2}' | cut -d'@' -f1); do
  sudo ip link del "$iface" 2>/dev/null || true
done
sudo iptables -t nat -F PREROUTING
sudo iptables -t nat -F POSTROUTING

export PATH=$PATH:/usr/sbin:/sbin:/usr/local/go/bin

BIN=./pocket-docker
ROOTFS=./busybox.tar

if [ ! -f "$BIN" ]; then
  go build -o pocket-docker ./cmd/pocket-docker
fi

ID=$(sudo env "PATH=$PATH" $BIN run -d --rootfs "$ROOTFS" --cmd "/bin/sh -c 'while true; do echo -e \"HTTP/1.1 200 OK\\r\\n\\r\\nhi\" | nc -l -p 80; done'" -p 8080:80 --network)

echo "Container $ID started"
# give container time
sleep 3

resp=$(curl -s http://localhost:8080)
if [[ "$resp" != "hi" ]]; then
  echo "unexpected response: $resp"
  sudo $BIN stop "$ID"
  exit 1
fi

sudo $BIN stop "$ID"
echo "network test passed"