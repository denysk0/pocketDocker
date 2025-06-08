
#!/usr/bin/env bash
set -euo pipefail

# Check for sqlite3 dependency
if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "Please install sqlite3 (e.g. sudo apt-get install sqlite3)"
  exit 1
fi

# Ensure we have Go in PATH even under sudo
export PATH=$PATH:/usr/local/go/bin

# Build the binary if not present
if [ ! -f ./pocket-docker ]; then
  go build -o pocket-docker ./cmd/pocket-docker
fi

#
# Run a long-lived container (sleep) to allow nsenter checks
CONTAINER_ID=$(sudo -E ./pocket-docker run --rootfs ./busybox.tar --cmd "/bin/sh -c 'sleep 5'")
echo "Container ID: $CONTAINER_ID"
# Retrieve actual PID from sqlite store
DB="$HOME/.pocket-docker/state.db"
CONTAINER_PID=$(sqlite3 "$DB" "SELECT pid FROM containers WHERE id='$CONTAINER_ID';")
echo "Container PID: $CONTAINER_PID"

# Give the container time to initialize namespaces
sleep 5

# Проверяем, что за пределами контейнера этот PID не виден
if ps -p "$CONTAINER_PID" > /dev/null 2>&1; then
  echo "Ошибка: процесс $CONTAINER_PID виден в основной системе."
  exit 1
else
  echo "Процесс $CONTAINER_PID изолирован."
fi

if command -v nsenter >/dev/null 2>&1; then
  NS_PATH="/proc/${CONTAINER_PID}/ns/pid"
  if [ -e "$NS_PATH" ]; then
    # Заходим внутрь контейнера и проверяем /proc
    nsenter --target "$CONTAINER_PID" --pid --mount ls /proc/1
  else
    echo "Skipping nsenter check: $NS_PATH not found"
  fi
else
  echo "nsenter not found, skipping nsenter check"
fi

# Stop the container via CLI
sudo ./pocket-docker stop "$CONTAINER_ID"
echo "Container $CONTAINER_ID stopped via CLI"