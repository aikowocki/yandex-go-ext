#!/bin/sh
set -eu

CLUSTER_NAME="${K3D_CLUSTER_NAME:-gophprofile}"
SYSTEM_NAMESPACE=kube-system
TRAEFIK_CHART_VERSION="${TRAEFIK_CHART_VERSION:-41.5.0}"

wait_for_resource() {
  resource="$1"
  name="$2"
  timeout="${3:-180}"
  elapsed=0

  while ! kubectl get "$resource" "$name" -n "$SYSTEM_NAMESPACE" >/dev/null 2>&1; do
    if [ "$elapsed" -ge "$timeout" ]; then
      echo "Timed out waiting for $resource/$name" >&2
      kubectl get events -n "$SYSTEM_NAMESPACE" --sort-by=.lastTimestamp | tail -20 >&2 || true
      return 1
    fi
    sleep 2
    elapsed=$((elapsed + 2))
  done
}

if ! k3d cluster list --no-headers | awk '{print $1}' | grep -qx "$CLUSTER_NAME"; then
  k3d cluster create "$CLUSTER_NAME" \
    --servers 1 \
    --agents 1 \
    --k3s-arg "--disable=traefik@server:0" \
    --port "8080:80@loadbalancer" \
    --port "8443:443@loadbalancer" \
    --port "30002:30002@loadbalancer" \
    --port "30003:30003@loadbalancer"
fi

kubectl config use-context "k3d-${CLUSTER_NAME}"
kubectl wait --for=condition=Ready node --all --timeout=180s

# Traefik устанавливается после готовности Kubernetes. Чтобы избежать
# race condition между встроенными k3s HelmChart тасками для CRDs и контроллером.
helm repo add traefik https://traefik.github.io/charts >/dev/null 2>&1 || true
helm repo update traefik
helm upgrade --install traefik traefik/traefik \
  --version "$TRAEFIK_CHART_VERSION" \
  --namespace "$SYSTEM_NAMESPACE" \
  --set providers.kubernetesCRD.enabled=true \
  --set ingressClass.enabled=true \
  --set ingressClass.isDefaultClass=true \
  --wait \
  --timeout 5m

kubectl wait --for=condition=Available deployment/traefik -n "$SYSTEM_NAMESPACE" --timeout=180s
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=traefik -n "$SYSTEM_NAMESPACE" --timeout=180s
kubectl wait --for=condition=Ready pod -l k8s-app=metrics-server -n "$SYSTEM_NAMESPACE" --timeout=180s

kubectl get nodes
kubectl get pods -n "$SYSTEM_NAMESPACE"
