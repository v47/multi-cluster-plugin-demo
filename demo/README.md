# Multicluster-runtime demo (kubeconfig provider, two kind clusters)

One operator process reconciles `Widget` objects across **two** kind clusters.
When a `Widget` appears in a cluster, the operator writes a `ConfigMap` back into
**that same cluster** — routing by `req.ClusterName`.

```
┌─────────────────────────── your host ───────────────────────────┐
│  operator (go run / ./bin/manager)                               │
│     │  connects to mcr-a for its manager + watches Secrets       │
│     │  member kubeconfigs come from labeled Secrets in mcr-a     │
└─────┼──────────────────────────┬─────────────────────────────────┘
      │ 127.0.0.1:port            │ 127.0.0.1:port
   ┌──▼── kind: mcr-a ──┐      ┌──▼── kind: mcr-b ──┐
   │ Widget → ConfigMap │      │ Widget → ConfigMap │
   └────────────────────┘      └────────────────────┘
```

## Prerequisites

`go` (1.26+), `docker`, `kind`, `kubectl`. The Widget CRD, controller and the
patched `cmd/main.go` are already in this repo.

## Run

```bash
# 1. Two kind clusters + CRDs + register both as members (idempotent)
./demo/kind-up.sh

# 2. Run the operator on the host (leave running; Ctrl-C to stop)
./demo/run-operator.sh

# 3. In another terminal: create a Widget in each cluster and verify
./demo/demo.sh

# When done
./demo/kind-down.sh
```

Expected `demo.sh` tail:

```
--- mcr-a --- {"cluster":"mcr-a","message":"hi from mcr-a","widget":"hello"}
--- mcr-b --- {"cluster":"mcr-b","message":"hi from mcr-b","widget":"hello"}
```

## How clusters are registered

The `kubeconfig` provider watches Secrets in `mcr-a` (namespace `default`) that
carry the label `sigs.k8s.io/multicluster-runtime-kubeconfig: "true"`. Each such
Secret holds a member cluster's kubeconfig under the `kubeconfig` key, and the
**Secret name becomes the cluster name** (`req.ClusterName`). `kind-up.sh`
creates one Secret per cluster (`mcr-a`, `mcr-b`).

Add a cluster at runtime by creating another labeled Secret; remove one by
deleting its Secret — no operator restart needed.

See the full write-up in [`../ARTICLE.md`](../ARTICLE.md).
