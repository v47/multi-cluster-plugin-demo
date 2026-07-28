#!/usr/bin/env bash
#
# Build and run the operator ON THE HOST, pointed at cluster A.
#
# It runs on the host (not inside a cluster) on purpose: kind API servers are
# published on 127.0.0.1:<port>, which is reachable from the host but not from a
# pod. The operator connects to cluster A for its own manager and to watch the
# kubeconfig Secrets; the member clusters are reached via those Secrets.
#
# Ctrl-C to stop.
#
source "$(dirname "$0")/common.sh"

echo "==> building operator"
( cd "$REPO" && go build -o bin/manager ./cmd )

echo "==> running operator against '$CLUSTER_A' (watching Secrets in ns '$SECRET_NS')"
echo
KUBECONFIG="$WORKDIR/${CLUSTER_A}.kubeconfig" exec "$REPO/bin/manager" \
  --kubeconfig-secret-namespace="$SECRET_NS" \
  --metrics-bind-address=0 \
  --health-probe-bind-address=:8082 \
  --leader-elect=false
