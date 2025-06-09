#!/bin/bash
# Debug script for Ubuntu VM
# Run: scp debug_ubuntu.sh denyak@192.168.64.2:~/Documents/pocketDocker/
# Then: ssh denyak@192.168.64.2 "cd Documents/pocketDocker && ./debug_ubuntu.sh"

echo "=== Debugging pocket-docker on Ubuntu ==="

echo "1. Environment check:"
echo "User: $(whoami)"
echo "Kernel: $(uname -r)"
echo "Cgroups v2: $(test -d /sys/fs/cgroup/unified && echo 'yes' || echo 'no')"
echo "User namespaces: $(cat /proc/sys/user/max_user_namespaces 2>/dev/null || echo 'disabled')"

echo -e "\n2. Required files:"
ls -la busybox.tar 2>/dev/null && echo "✓ busybox.tar exists" || echo "✗ busybox.tar missing"

echo -e "\n3. Testing individual components with 20s timeout:"

echo "Build test:"
timeout 20 go build -o pocket-docker ./cmd/pocket-docker
echo "Build exit code: $?"

echo -e "\nCgroups test:"
timeout 20 go test -v ./internal/runtime/cgroups
echo "Cgroups exit code: $?"

echo -e "\nNetwork unit test:"
timeout 20 go test -v ./internal/runtime -run TestSetupNetworkingCommands
echo "Network unit exit code: $?"

echo -e "\nWatchdog test:"
timeout 20 go test -v ./internal/runtime -run TestWatchdog
echo "Watchdog exit code: $?"

echo -e "\nUser namespace test (likely to hang):"
timeout 20 go test -v ./internal/runtime -run TestCloneAndRunUserNamespace
echo "UserNS exit code: $?"

echo -e "\n4. Minimal namespace test:"
timeout 10 unshare --user --pid --mount bash -c 'echo "Namespace test OK"' 2>/dev/null
echo "Manual namespace exit code: $?"

echo -e "\n5. Check for busybox extraction:"
if [ -f busybox.tar ]; then
    tmpdir=$(mktemp -d)
    timeout 10 tar -tf busybox.tar | head -5
    echo "Tar listing exit code: $?"
    rm -rf "$tmpdir"
fi

echo -e "\nDone. Check which tests hang and their exit codes."