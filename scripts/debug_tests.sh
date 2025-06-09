#!/bin/bash

echo "=== Debugging pocket-docker tests ==="

# Check required files
echo "1. Checking busybox.tar..."
if [ -f "busybox.tar" ]; then
    echo "✓ busybox.tar exists"
else
    echo "✗ busybox.tar missing"
fi

# Test individual test files with timeout
echo -e "\n2. Testing individual files with 30s timeout..."

echo "Testing cgroups..."
timeout 30 go test -v ./internal/runtime/cgroups -run TestApplyMemoryAndCPU
echo "Exit code: $?"

echo -e "\nTesting network (unit test only)..."
timeout 30 go test -v ./internal/runtime -run TestSetupNetworkingCommands
echo "Exit code: $?"

echo -e "\nTesting watchdog..."
timeout 30 go test -v ./internal/runtime -run TestWatchdog
echo "Exit code: $?"

echo -e "\nTesting user namespace (may hang)..."
timeout 30 go test -v ./internal/runtime -run TestCloneAndRunUserNamespace
echo "Exit code: $?"

echo -e "\n3. Checking permissions..."
echo "Current user: $(whoami)"
echo "Can create namespaces: $(unshare --user --pid echo 'yes' 2>/dev/null || echo 'no')"

echo -e "\n4. Checking build..."
timeout 10 go build -o pocket-docker ./cmd/pocket-docker
if [ $? -eq 0 ]; then
    echo "✓ Build successful"
else
    echo "✗ Build failed"
fi