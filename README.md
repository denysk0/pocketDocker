# Pocket-Docker — a minimal, daemon-free container runtime


Pocket-Docker is a single-binary container runtime (~5 MB, CGO-free) that demonstrates how a container can be built from core Linux primitives:

- **Namespaces**: PID, UTS, mount, network, user  
- **cgroups v2**: CPU and memory limits, OOM watcher and auto-restart  
- **pivot_root** for isolated root filesystems  
- **SQLite (WAL)** for container metadata

This runtime is suitable for system programming courses, CTF exercises, embedded and edge environments where a full Docker stack is not required.

---

## Features

- **Small footprint**  
  – binary size ≤ 6 MB, no external daemon  
- **Fast build**  
  – `go build` completes in ≲ 5 s  
- **Rootless by default**  
  – unprivileged user namespaces  
- **Resource limits and health checks**  
  – memory/CPU via cgroups, optional seccomp profiles  
- **Persistent state**  
  – SQLite database with migrations and WAL mode  
- **Simple CLI**  
  – commands: `run`, `stop`, `ps`, `logs`, `exec`, `pull`, `rm`

---

## Quick start

```bash
# Build the binary
go build -o pocket-docker ./cmd/pocket-docker

# Pull a minimal Alpine rootfs
./pocket-docker pull \
  https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/x86_64/alpine-minirootfs-3.19.1-x86_64.tar.gz

# Run a container that prints a message
ID=$(./pocket-docker run \
  --rootfs alpine-minirootfs-3.19.1-x86_64.tar.gz \
  --cmd "/bin/sh -c 'echo \"Hello from container\"'")

# Inspect output and status
./pocket-docker logs "$ID"
./pocket-docker ps
./pocket-docker stop "$ID"
```

⸻

Demonstration examples

1. Python “Hello, world”
```
# Pull a Python-enabled image (prebuilt tar)
./pocket-docker pull https://example.com/python3.12-alpine.tar

# Run a Python one-liner
./pocket-docker run \
  --rootfs python3.12-alpine.tar \
  --cmd "python -c 'print(\"Hello, world\")'"
```
Expected output:
Hello, world

2. Simple test-runner for coding exercises

Given a Python program main.py:

n = int(input())
print(n * n)

And test files:
	•	cases/in.txt contains 5
	•	cases/out.txt contains 25

Run inside the container and compare:
```
./pocket-docker run \
  --rootfs python3.12-alpine.tar \
  --volume "$(pwd)/cases:/mnt:ro" \
  --cmd "/bin/sh -c 'python /mnt/main.py < /mnt/in.txt > /mnt/out && diff -u /mnt/out.txt /mnt/out'"
```
Exit status 0 indicates success; non-zero indicates a difference.

