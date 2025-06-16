#!/usr/bin/env bash
set -euo pipefail

# ————————————————————————————————————————————————
# Настройки VM и путей
VM_HOST="denyak@192.168.64.2"
VM_PATH="/home/denyak/Documents/pocketDocker"
LOCAL_PATH="/Users/denyak/Documents/projects/pocketDocker"

# SSH-опции для мультиплексирования (ControlMaster)
SSH_OPTS=(
  -o ControlMaster=auto
  -o ControlPersist=600s
  -o ControlPath="$HOME/.ssh/cm-%r@%h:%p"
)

# ————————————————————————————————————————————————
# Открываем master-коннект (спросит пароль один раз)
ssh "${SSH_OPTS[@]}" -Nf "$VM_HOST" || {
  echo "Не удалось установить мастер-соединение"
  exit 1
}

# на выходе закрыть мастер-коннект
trap 'ssh "${SSH_OPTS[@]}" -O exit "$VM_HOST" >/dev/null 2>&1' EXIT

echo "=== Sync: cmd, internal, scripts → $VM_HOST:$VM_PATH ==="

# Создать целевую директорию
ssh "${SSH_OPTS[@]}" "$VM_HOST" "mkdir -p '$VM_PATH'"

# Синхронизируем только нужные каталоги
for dir in cmd internal scripts; do
  if [ -d "$LOCAL_PATH/$dir" ]; then
    echo "📁 Sync $dir..."
    rsync -avz --delete \
      -e "ssh ${SSH_OPTS[*]}" \
      "$LOCAL_PATH/$dir/" \
      "$VM_HOST:$VM_PATH/$dir/"
  else
    echo "⚠️  $dir не найден, пропускаем"
  fi
done