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

helm repo add harbor https://helm.goharbor.io >/dev/null 2>&1 || true
helm repo update harbor
helm upgrade --install "$RELEASE" harbor/harbor \
  --version "$CHART_VERSION" \
  --namespace "$NAMESPACE" \
  --create-namespace \
  --wait \
  --timeout 15m \
  -f "$VALUES_FILE" \
  --set-string harborAdminPassword="$HARBOR_ADMIN_PASSWORD"

kubectl get pods -n "$NAMESPACE"
kubectl get svc -n "$NAMESPACE" harbor

if [ "${HARBOR_CONFIGURE:-1}" = 1 ]; then
  HARBOR_URL="${HARBOR_URL:-http://localhost:30002}" \
    HARBOR_PROJECT="${HARBOR_PROJECT:-gophprofile}" \
    sh "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/configure-harbor.sh"
fi
