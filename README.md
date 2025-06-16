# pocket-docker

A tiny, self-contained container runtime written in Go.  
It compiles to a single binary, uses only the kernel’s own namespaces/cgroups, and stores all state in `~/.pocket-docker/state.db`.

---

## Table of Contents
1. [Requirements](#requirements)
2. [Preparing Root-File-System TARs](#preparing-root-file-system-tars)
3. [Building pocket-docker](#building-pocket-docker)
4. [Running the Test-Suite](#running-the-test-suite)
5. [Quick Tour](#quick-tour)  
   5.1  [Creating a BusyBox Shell](#creating-a-busybox-shell)  
   5.2  [Running a long-lived BusyBox loop](#running-a-long-lived-busybox-loop)  
   5.3  [Inspect, Exec, Stop, Remove](#inspect-exec-stop-remove)  
6. [FAQ / Tips](#faq--tips)

---

## Requirements

| Needed for…            | Tool / Kernel feature | How to install / enable                                      |
|------------------------|-----------------------|--------------------------------------------------------------|
| **Compile**            | Go 1.22+              | <https://go.dev/dl/>                                         |
| **Networking support** | `ip` ( iproute2 )     | `sudo apt install iproute2` / `apk add iproute2`             |
|                        | `iptables`            | `sudo apt install iptables` / `apk add iptables`             |
| **Image unpacking**    | `tar`                 | Already on most systems                                      |
| **Tests**              | Everything above plus no-root user; they mock out privileged ops |

pocket-docker runs fine **root-less** *unless* you ask for `--network` or port publishing, which require root/CAP\_NET\_ADMIN.

| cgroup v2 only?            | Yes. Most modern distros enable it by default; if not, boot with `systemd.unified_cgroup_hierarchy=1`. |

### Getting a rootfs without Docker

Any `.tar` that expands to a Linux filesystem will do. We use Docker only because it’s a convenient image source:

```bash
# install the daemon once
sudo apt install docker.io          # Debian/Ubuntu
# or: brew install docker && open Docker.app  # macOS

# pull and export BusyBox
docker pull busybox:latest
docker export $(docker create busybox) > busybox.tar
```

You can equally well hand-craft a directory and `tar -cf myrootfs.tar .`, or download tarballs from elsewhere.

---

## Preparing Root-File-System TARs

Any directory or OCI image can be turned into a rootfs tarball.  
The examples below use Docker because it is ubiquitous, but Podman works the same.

### BusyBox

```bash
docker pull busybox:latest
ctr=$(docker create busybox)
docker export "$ctr" > busybox.tar
docker rm "$ctr"
```
Python 3.12-Alpine
```bash
docker pull python:3.12-alpine
ctr=$(docker create python:3.12-alpine)
docker export "$ctr" > python312-alpine.tar
docker rm "$ctr"
```

Copy the resulting *.tar files anywhere you like (commonly under ~/images/).

---

## Building pocket-docker

```bash
git clone https://github.com/yourname/pocket-docker.git
cd pocket-docker
go build -o pocket-docker
```

The single binary embeds no assets; move it anywhere on $PATH.

---

## Running the Test-Suite

```bash
go test ./...
```

All tests run as an ordinary user.
SKIP_SETUP=1 go test ./... disables the heavy rootfs setup during integration tests if you only need fast unit coverage.

---

## Quick Tour

### Creating a BusyBox Shell

```bash
# One-shot interactive container
```bash
sudo ./pocket-docker run \
  --rootfs busybox.tar \
  --cmd "/bin/sh" \
  --interactive -t \
  --network \
  --publish 8080:80 \
  --memory 104857600         # 100 MiB
```
What happens:
	•	Extracts busybox.tar to a temp dir.
	•	Creates new mount/uts/pid/net/user namespaces.
	•	Applies a 100 MiB cgroup-v2 memory limit.
	•	Sets up a veth pair (10.42.0.x) and DNATs host :8080 → container :80.
	•	Drops you into a BusyBox shell attached to the container’s PTY.

Detach with Ctrl-P Ctrl-Q / use --detach / type `exit` into shell.

---

### Running a long-lived BusyBox loop

Need a container that just stays alive? Run an endless shell loop:

```bash
sudo ./pocket-docker run \
  --rootfs busybox.tar \
  --cmd "/bin/sh -c 'while true; do sleep 3600; done'" \
  --detach
```

*Why the `sudo`?*  
Any use of `--network` or `--publish` needs `CAP_NET_ADMIN`; the easiest way to grant that is simply to run the command with `sudo`.

---

### Run vs Exec: interactive options

`pocket-docker run` can start a brand-new container in interactive mode (`-i -t`).  
`pocket-docker exec` jumps **into** a running container.

| flags in exec | behaviour inside container                                | how to get back to host                                          |
|---------------|-----------------------------------------------------------|------------------------------------------------------------------|
| `-i -t`       | full TTY + job-control; BusyBox prints a harmless warning | host shell **often hangs**; kill the `exec` process from another tee |
| `-i`          | stdin forwarded, no new TTY; works for most commands      | clean `exit`/`Ctrl-D` returns to host                            |
| `-t`          | TTY but no stdin – rarely useful by itself                | —                                                                |

For everyday debugging, use:

```bash
./pocket-docker exec -i <ID> /bin/sh
```

…and you can `exit` normally. Reserve `-it` for full-screen apps (`vi`, `htop`, etc.).

> **Note:** You may sometimes see a message like `/bin/sh: 1: Cannot set tty process group (No such process)` after exiting an `exec` session. This is a harmless BusyBox job-control warning and your host terminal will be restored correctly.
---

### Inspect, Exec, Stop, Remove

```bash
# List containers
```bash
./pocket-docker ps
```
Lets say we got an example ID: `9c8d5b9e3ab24739a13f5be4c9a5b6c1`
# Exec into a running container
```bash
./pocket-docker exec -i -t 9c8d5b9e3ab24739a13f5be4c9a5b6c1 /bin/sh
```
# Follow logs
```bash
./pocket-docker logs -f --tail 50 9c8d5b9e3ab24739a13f5be4c9a5b6c1
```
# Stop (graceful, then SIGKILL after 5 s)
```bash
./pocket-docker stop 9c8d5b9e3ab24739a13f5be4c9a5b6c1
```
# Remove record from state db
```bash
./pocket-docker rm 9c8d5b9e3ab24739a13f5be4c9a5b6c1
```
Need them all?
```bash
./pocket-docker stop --all
./pocket-docker rm   --all
```

---

## FAQ / Tips

---

| Question                   | Answer                                                                                         |
|----------------------------|------------------------------------------------------------------------------------------------|
| Where are images stored?   | Anywhere—the `--rootfs` flag can point to a tarball or a directory. The pull command caches under `~/.pocket-docker/images/`. |
| Logs?                      | Text files in `~/.pocket-docker/logs/<ID>.log`. `logs -f` tails them efficiently with back-off. |
| State DB?                  | `~/.pocket-docker/state.db` (SQLite WAL). If you sudo, ownership is handed back to the invoking user. |
| Cleaning temp rootfs dirs  | They are removed automatically during normal shutdown; in case of a crash, purge `/tmp/pocketdocker-rootfs-*`. |
| cgroup v2 only?            | Yes. Most modern distros enable it by default; if not, boot with `systemd.unified_cgroup_hierarchy=1`. |
| Why is `stop --all` slow? | It visits every running container, waits up to 5 s for each to gracefully shut down, then tears down cgroups, networking, and temp rootfs **one by one**. With many containers that sequential cleanup is noticeable. |