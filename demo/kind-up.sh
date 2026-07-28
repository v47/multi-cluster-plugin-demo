#!/usr/bin/env bash
#
# Create the two kind clusters, install the Widget CRD on both, and register
# them with the operator by creating one labeled kubeconfig Secret per cluster
# in cluster A. Idempotent: safe to re-run.
#
source "$(dirname "$0")/common.sh"

for c in "$CLUSTER_A" "$CLUSTER_B"; do
  if kind get clusters 2>/dev/null | grep -qx "$c"; then
    echo "==> kind cluster '$c' already exists"
  else
    echo "==> creating kind cluster '$c'"
    kind create cluster --name "$c" --wait 120s
  fi
done

echo "==> generating CRD manifests"
( cd "$REPO" && make manifests >/dev/null )

CRD="$REPO/config/crd/bases/demo.example.com_widgets.yaml"
echo "==> installing Widget CRD on both clusters"
kubectl --context "$CTX_A" apply -f "$CRD"
kubectl --context "$CTX_B" apply -f "$CRD"

echo "==> exporting per-cluster kubeconfigs to $WORKDIR"
mkdir -p "$WORKDIR"
kind get kubeconfig --name "$CLUSTER_A" > "$WORKDIR/${CLUSTER_A}.kubeconfig"
kind get kubeconfig --name "$CLUSTER_B" > "$WORKDIR/${CLUSTER_B}.kubeconfig"

echo "==> registering member clusters as labeled Secrets in '$CLUSTER_A' (ns: $SECRET_NS)"
for c in "$CLUSTER_A" "$CLUSTER_B"; do
  # The Secret name becomes the multicluster ClusterName (req.ClusterName).
  kubectl --context "$CTX_A" -n "$SECRET_NS" delete secret "$c" --ignore-not-found >/dev/null
  kubectl --context "$CTX_A" -n "$SECRET_NS" create secret generic "$c" \
    --from-file=kubeconfig="$WORKDIR/${c}.kubeconfig" >/dev/null
  kubectl --context "$CTX_A" -n "$SECRET_NS" label secret "$c" \
    "${KUBECONFIG_LABEL}=true" --overwrite >/dev/null
done

echo
echo "==> member clusters registered:"
kubectl --context "$CTX_A" -n "$SECRET_NS" get secrets -l "${KUBECONFIG_LABEL}=true"
echo
echo "Next: ./demo/run-operator.sh   (in one terminal)"
echo "Then: ./demo/demo.sh           (in another)"
