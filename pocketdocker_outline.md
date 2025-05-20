PocketDocker Project Overview

Functional Requirements (Minimal MVP):
	1.	FR-1: CLI command run — takes an image name/path (rootfs-tar), creates a new container isolated via namespaces (PID, UTS, Mount, Network), mounts rootfs using pivot_root.
	2.	FR-2: CLI command stop — sends SIGTERM, waits up to 5 seconds, then SIGKILL if necessary; cleans up cgroups and mounts.
	3.	FR-3: CLI command ps — displays a table of all containers: ID, name, image, status (Running/Stopped), and start time.
	4.	FR-4: Resource limits via cgroups v2: memory (RAM limit) and CPU shares.
	5.	FR-5: Metadata storage using SQLite: tables for containers and images, basic CRUD API.
	6.	FR-6: Command pull — downloads and unpacks the image tar (~OCI manifest) into ~/.pocket-docker/images.
	7.	FR-7 (stretch): Command logs — multiplexes container stdout/stderr into log files and allows real-time reading.

Non-Functional Requirements:
	•	NFR-1: Performance: container startup under 3 seconds, ps command under 1 second.
	•	NFR-2: Reliability: automatic restarts on failures (health-check), ensuring 99% uptime.
	•	NFR-3: Security: full isolation via namespaces, minimal privileges (non-root inside).

Technology Stack:
	•	Language: Go 1.22
	•	CLI Framework: spf13/cobra
	•	Storage: SQLite (pure-Go library)
	•	Cgroups: v2 unified
	•	Filesystem: pivot_root + mount /proc, /sys
	•	Logs: PTY multiplexer → logs stored at ~/.pocket-docker/logs
	•	CI/Testing: GitHub Actions, go test, e2e tests with Bats

⸻

Development Roadmap

Phase 1: Project Skeleton & CLI Framework
	•	Initialize repo pocket-docker with go mod init.
	•	Set up cmd/pocket-docker/main.go using cobra.Command.
	•	Register stubs for commands run, stop, ps, pull, logs.
	•	Each command outputs TODO: <command> and exits with status code 0.
	•	Verify: go build ./cmd/pocket-docker produces a working binary without panics.

Phase 2: Process Isolation
	•	Implement container launch logic using syscall.Clone with namespaces and pivot_root in internal/runtime/isolate.go.
	•	Test launching a simple /bin/sh shell in an isolated namespace.

Phase 3: Resource Limits with cgroups v2
	•	Create CPU and memory controllers in internal/runtime/cgroups.go, associate container PID.
	•	Handle OOM events (via memory.events) with automatic termination.

Phase 4: Metadata Management & ps Command
	•	Set up SQLite schema (containers, images) in internal/store/.
	•	Implement CRUD and container listing logic in internal/cli/ps.go.

Phase 5: Container Shutdown
	•	Implement Stop() method in internal/runtime/container.go: SIGTERM → timeout → SIGKILL → cleanup.
	•	Wrap with CLI in internal/cli/stop.go.

Phase 6: Image Pulling
	•	Download image tar, verify SHA, unpack to ~/.pocket-docker/images, and store metadata in internal/cli/pull.go and internal/store/image.go.

Phase 7: Logging and Health-check (Stretch Goal)
	•	Implement PTY multiplexer to log files.
	•	Command pocketdocker logs <id> reads latest logs and listens for new entries.
	•	Basic health probe to check PID liveness and auto-restart failed containers.

Phase 8: Network Namespace & Scaling (Optional)
	•	Setup veth-pairs, bridges, and port-forwarding.
	•	Develop basic scheduling with a master/agent architecture.

Phase 9: Completion — Testing, CI, Documentation
	•	Write unit and end-to-end tests; configure GitHub Actions.
	•	Finalize README with usage examples and NFR metrics.