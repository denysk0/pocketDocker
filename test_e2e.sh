#!/usr/bin/env bash
set -euo pipefail

# Собираем бинарь
go build -o pocket-docker ./cmd/pocket-docker

# Запускаем контейнер на BusyBox
CONTAINER_PID=$(./pocket-docker run --rootfs ./busybox.tar --cmd "/bin/sh")
echo "Container PID: $CONTAINER_PID"

# Даем время на запуск
sleep 1

# Проверяем, что за пределами контейнера этот PID не виден
if ps -p "$CONTAINER_PID" > /dev/null 2>&1; then
  echo "Ошибка: процесс $CONTAINER_PID виден в основной системе."
  exit 1
else
  echo "Процесс $CONTAINER_PID изолирован."
fi

# Заходим внутрь контейнера и проверяем /proc
nsenter --target "$CONTAINER_PID" --pid --mount ls /proc/1

# Убиваем контейнер
kill "$CONTAINER_PID"
echo "Контейнер остановлен."