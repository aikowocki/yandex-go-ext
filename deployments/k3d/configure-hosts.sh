#!/bin/bash
set -eu

HOSTS=(
  "127.0.0.1 gophprofile.localhost"
  "127.0.0.1 grafana.gophprofile.localhost"
  "127.0.0.1 prometheus.gophprofile.localhost"
  "127.0.0.1 alertmanager.gophprofile.localhost"
  "127.0.0.1 vault.gophprofile.localhost"
  "127.0.0.1 jaeger.gophprofile.localhost"
  "127.0.0.1 harbor.gophprofile.localhost"
)

HOSTS_FILE="/etc/hosts"
MARKER="# GophProfile k3d cluster"

echo "Конфигурировнаие /etc/hosts для GophProfile сервисов..."

if grep -q "$MARKER" "$HOSTS_FILE"; then
  echo "✓ GophProfile hosts уже добавлены в $HOSTS_FILE"
  exit 0
fi

{
  echo ""
  echo "$MARKER"
  for host in "${HOSTS[@]}"; do
    echo "$host"
  done
} | sudo tee -a "$HOSTS_FILE" > /dev/null

echo "✓ Добавлены GophProfile хосты в $HOSTS_FILE"
echo ""
grep "$MARKER" -A 10 "$HOSTS_FILE"
