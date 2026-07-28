#!/usr/bin/env bash
#
# Apply a Widget into each cluster and show that the single operator reconciled
# both — writing a ConfigMap into whichever cluster the Widget appeared in.
#
# Run ./demo/run-operator.sh in another terminal first.
#
source "$(dirname "$0")/common.sh"

echo "==> applying a Widget into each cluster"
sed "s/__CLUSTER__/$CLUSTER_A/" "$REPO/demo/widget.tmpl.yaml" | kubectl --context "$CTX_A" apply -f -
sed "s/__CLUSTER__/$CLUSTER_B/" "$REPO/demo/widget.tmpl.yaml" | kubectl --context "$CTX_B" apply -f -

echo
echo "==> waiting for the operator to set .status.observedCluster"
kubectl --context "$CTX_A" wait widget/hello \
  --for=jsonpath='{.status.observedCluster}'="$CLUSTER_A" --timeout=90s
kubectl --context "$CTX_B" wait widget/hello \
  --for=jsonpath='{.status.observedCluster}'="$CLUSTER_B" --timeout=90s

echo
echo "==> Widgets (note the Observed-Cluster column differs per cluster)"
echo "--- $CLUSTER_A ---"; kubectl --context "$CTX_A" get widgets
echo "--- $CLUSTER_B ---"; kubectl --context "$CTX_B" get widgets

echo
echo "==> ConfigMaps the single operator wrote into each cluster"
echo -n "--- $CLUSTER_A --- "; kubectl --context "$CTX_A" get configmap widget-hello -o jsonpath='{.data}'; echo
echo -n "--- $CLUSTER_B --- "; kubectl --context "$CTX_B" get configmap widget-hello -o jsonpath='{.data}'; echo

echo
echo "One operator process, two clusters: req.ClusterName routed each write to the right cluster."
