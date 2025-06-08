# pocketDocker
Docker-like CLI tool in Go

# Quick start
```bash
# build
go build -o pocket-docker ./cmd/pocket-docker

# pull a tiny image (busybox.tar downloaded beforehand)
# run container with 64 MB RAM and default CPU weight
sudo ./pocket-docker run --rootfs busybox.tar --cmd "/bin/sh -c 'echo hello'" --memory 67108864

# list containers
./pocket-docker ps

# stop container
sudo ./pocket-docker stop <ID>
```