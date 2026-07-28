# Shared configuration for the multicluster-runtime demo scripts.
# Sourced by the other scripts in this directory.
set -euo pipefail

# Repository root (parent of this demo/ directory).
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Where generated per-cluster kubeconfigs are written (git-ignored).
WORKDIR="$REPO/.work"

# The two kind clusters. Cluster A doubles as the "registry" (it holds the
# kubeconfig Secrets the operator watches) and as a member; cluster B is a member.
CLUSTER_A="mcr-a"
CLUSTER_B="mcr-b"
CTX_A="kind-${CLUSTER_A}"
CTX_B="kind-${CLUSTER_B}"

# Namespace in cluster A where the labeled kubeconfig Secrets live.
SECRET_NS="default"

# The label the kubeconfig provider looks for. It must have the value "true".
KUBECONFIG_LABEL="sigs.k8s.io/multicluster-runtime-kubeconfig"
