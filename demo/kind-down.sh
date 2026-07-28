#!/usr/bin/env bash
#
# Tear down both kind clusters and remove generated kubeconfigs.
#
source "$(dirname "$0")/common.sh"

for c in "$CLUSTER_A" "$CLUSTER_B"; do
  echo "==> deleting kind cluster '$c'"
  kind delete cluster --name "$c" || true
done

rm -rf "$WORKDIR"
echo "==> done (clusters deleted, $WORKDIR removed)"
