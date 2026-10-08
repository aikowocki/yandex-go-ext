#!/usr/bin/env bash
set -euo pipefail

# Проверка синтаксиса Prometheus alert rules через promtool

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
RULES_DIR="${PROJECT_ROOT}/deployments/observability/rules"

echo "🔍 Проверка синтаксиса alert rules..."

docker run --rm \
  -v "${RULES_DIR}:/rules:ro" \
  --entrypoint promtool \
  prom/prometheus:latest \
  check rules /rules/gophprofile-alerts.yaml

echo "✅ Alert rules корректны"
