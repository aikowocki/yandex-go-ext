#!/bin/sh
set -eu

NAMESPACE="${HARBOR_NAMESPACE:-harbor}"
RELEASE="${HARBOR_RELEASE:-harbor}"
CHART_VERSION="${HARBOR_CHART_VERSION:-1.19.2}"
VALUES_FILE="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/helm/harbor-values-dev.yaml"

if [ -z "${HARBOR_ADMIN_PASSWORD:-}" ]; then
  if [ ! -t 0 ]; then
    echo "HARBOR_ADMIN_PASSWORD обязателен для неинтерактивного использования" >&2
    exit 1
  fi

  printf 'Пароль администратора Harbor: '
  stty -echo
  trap 'stty echo 2>/dev/null || true; printf "\n"' EXIT INT TERM
  IFS= read -r HARBOR_ADMIN_PASSWORD
  stty echo
  trap - EXIT INT TERM
  printf '\n'
  export HARBOR_ADMIN_PASSWORD
fi

echo "=== Удаление старого Harbor и очистка БД ==="
helm uninstall "$RELEASE" -n "$NAMESPACE" 2>/dev/null || true
kubectl delete ns "$NAMESPACE" 2>/dev/null || true

echo "=== Ожидание удаления namespace ==="
for i in $(seq 1 30); do
  if ! kubectl get ns "$NAMESPACE" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "=== Установка нового Harbor с пароль=$HARBOR_ADMIN_PASSWORD ==="
helm repo add harbor https://helm.goharbor.io >/dev/null 2>&1 || true
helm repo update harbor

helm install "$RELEASE" harbor/harbor \
  --version "$CHART_VERSION" \
  --namespace "$NAMESPACE" \
  --create-namespace \
  --wait \
  --timeout 15m \
  -f "$VALUES_FILE" \
  --set-string harborAdminPassword="$HARBOR_ADMIN_PASSWORD"

echo ""
echo "=== Статус подов Harbor ==="
kubectl get pods -n "$NAMESPACE"

echo ""
echo "=== Сервис Harbor ==="
kubectl get svc -n "$NAMESPACE" harbor

echo ""
echo "=== Ожидание инициализации Harbor API ==="
sleep 10

if [ "${HARBOR_CONFIGURE:-1}" = 1 ]; then
  HARBOR_URL="${HARBOR_URL:-http://localhost:30002}" \
    HARBOR_PROJECT="${HARBOR_PROJECT:-gophprofile}" \
    HARBOR_ADMIN_PASSWORD="$HARBOR_ADMIN_PASSWORD" \
    bash "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/configure-harbor.sh"
fi

echo ""
echo "=== Harbor установлен и настроен ==="
echo "URL: http://localhost:30002"
echo "Пользователь: admin"
echo "Пароль: $HARBOR_ADMIN_PASSWORD"
