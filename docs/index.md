---
title: One Operator, Many Clusters
description: Multicluster controllers with Kubebuilder and multicluster-runtime
---

## The problem

Almost every non‑trivial Kubernetes shop ends up with **more than one cluster**:
prod and staging, one per region, one per tenant, or an ephemeral fleet that
grows and shrinks during the day. The moment you want a controller to act across
those clusters, the standard toolkit pushes back.

A normal controller built with [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
is **single‑cluster by construction**. `ctrl.NewManager(cfg, ...)` binds to one
API server. Its client talks to one cluster. A `reconcile.Request` is just a
`NamespacedName` — it has no idea *which* cluster an event came from, because
there is only one. To go multi‑cluster you historically had to run N copies of
your operator, or hand‑roll a mesh of clients and informers and pray.

[**multicluster-runtime**](https://github.com/kubernetes-sigs/multicluster-runtime)
(a Kubernetes SIG project) fixes this at the framework level. It keeps the
controller‑runtime programming model you already know — builders, reconcilers,
caches — but makes the **cluster a first‑class dimension** of every request. And
[**Kubebuilder**](https://book.kubebuilder.io/) ships an (alpha) plugin that
scaffolds a project wired for it, so you don't start from a blank `main.go`.

In this article we'll:

1. Scaffold a project with the Kubebuilder `multicluster-runtime` plugin.
2. See the multicluster wiring the plugin generates for v0.24.1.
3. Write a tiny cross‑cluster controller.
4. Run it against **two `kind` clusters** and watch one binary reconcile both.

Everything runs locally. The finished demo is in this repo under [`demo/`](https://github.com/v47/multi-cluster-plugin-demo/tree/HEAD/demo).

Here's the punchline we'll build to — one operator process, two clusters, and a
`ConfigMap` written into whichever cluster the `Widget` appeared in:

```text
--- mcr-a --- {"cluster":"mcr-a","message":"hi from mcr-a","widget":"hello"}
--- mcr-b --- {"cluster":"mcr-b","message":"hi from mcr-b","widget":"hello"}
```

---

## The one idea that matters: `req.ClusterName`

multicluster-runtime introduces a **provider** — a component that discovers
clusters and *engages* them with the manager — and threads a **cluster name**
through the entire reconcile path.

Three types replace their single‑cluster cousins:

| single‑cluster (controller-runtime) | multicluster (multicluster-runtime) |
|---|---|
| `ctrl.Manager` | `mcmanager.Manager` |
| `reconcile.Request` (`NamespacedName`) | `mcreconcile.Request` (`NamespacedName` **+ `ClusterName`**) |
| `ctrl.NewControllerManagedBy(mgr)` | `mcbuilder.ControllerManagedBy(mgr)` |

Your reconciler no longer holds *the* client, because there is no single "the".
Instead it asks the manager for the right cluster at request time:

```go
cl, _ := r.Manager.GetCluster(ctx, req.ClusterName) // controller-runtime cluster.Cluster
c := cl.GetClient()                                 // a client for THAT cluster
```

The same controller is automatically wired to every cluster the provider
engages. That's the whole trick: **write a normal reconciler, get a fleet‑wide
controller.**

### Providers

How clusters are *discovered* is pluggable. multicluster-runtime ships several
providers; the Kubebuilder plugin can scaffold four of them:

| Provider | When to use it |
|---|---|
| **kubeconfig** | Dynamic fleet: clusters join/leave at runtime by creating kubeconfig `Secret`s. *(used here)* |
| **namespace** | Single cluster, one "virtual cluster" per namespace. |
| **cluster-api** | Fleet managed by [Cluster API](https://cluster-api.sigs.k8s.io/). |
| **file** | Static list: one kubeconfig file per cluster in a directory. |

We'll use the **kubeconfig** provider — it tells the best story and maps cleanly
onto `kind`.

---

## Prerequisites

- Go 1.26+
- Docker
- [`kind`](https://kind.sigs.k8s.io/) and `kubectl`
- A Kubebuilder binary **built with the `multicluster-runtime` plugin** (below)

Versions used while writing this: Go 1.26, `kind` v0.27, kubectl v1.33,
multicluster-runtime **v0.24.1**.

---

## Step 1 — A Kubebuilder with the plugin

The `multicluster-runtime` plugin is an in‑tree *optional* plugin. Build the CLI
from the branch that carries it:

```bash
git clone https://github.com/<you>/kubebuilder      # the branch with the plugin
cd kubebuilder
make build                                           # -> ./bin/kubebuilder
```

Confirm the plugin is registered (note the key — that's what you pass to
`--plugins`):

```bash
$ ./bin/kubebuilder init --plugins --help | grep multicluster
  multicluster-runtime.sigs.k8s/v1-alpha  External or custom plugin
```

---

## Step 2 — Scaffold the project

The plugin is designed to run **after** `go/v4` in the plugin chain. `go/v4` lays
down the standard project; `multicluster-runtime` rewrites `cmd/main.go` to use a
multicluster manager and generates a multicluster‑aware controller.

```bash
mkdir mcr-demo && cd mcr-demo

kubebuilder init \
  --plugins go/v4,multicluster-runtime.sigs.k8s/v1-alpha \
  --domain example.com \
  --repo github.com/you/mcr-demo \
  --provider kubeconfig

kubebuilder create api \
  --plugins go/v4,multicluster-runtime.sigs.k8s/v1-alpha \
  --group demo --version v1 --kind Widget \
  --controller --resource
```

Kubebuilder resolves the dependency for you during `init`:

```text
go: found sigs.k8s.io/multicluster-runtime/pkg/manager in sigs.k8s.io/multicluster-runtime v0.24.1
```

Look at what the plugin changed versus a vanilla project:

- `cmd/main.go` imports `mcmanager` and a provider, and calls `mcmanager.New(...)`
  instead of `ctrl.NewManager(...)`.
- `internal/controller/widget_controller.go` uses `mcreconcile.Request` and
  `mcbuilder.ControllerManagedBy(mgr)`.

That's a real head start — and because the plugin targets v0.24.1, it compiles as generated. Let's look at what it wired up, and why.

---

## Step 3 — What the plugin generates (and why)

The scaffolded `cmd/main.go` **compiles against v0.24.1 as generated** — no
patching required:

```text
$ go build ./...
$ echo $?
0
```

That wasn't always true (earlier iterations of the plugin emitted an older,
idealized API), so it's worth understanding *what* it produced — the multicluster
wiring differs from single-cluster controller-runtime in three places.

### 3a. The provider is constructed with your scheme

```go
provider := kubeconfigprovider.New(kubeconfigprovider.Options{
    Namespace: secretNamespace,
    ClusterOptions: []cluster.Option{
        func(o *cluster.Options) { o.Scheme = scheme }, // member clusters learn about Widget
    },
})
```

The provider builds each engaged cluster with `cluster.New(cfg, ClusterOptions...)`.
Injecting your scheme here is what lets the member clusters' caches and clients
understand your `Widget` CRD — omit it and you get a confusing "no kind is
registered for Widget".

### 3b. The provider is the second argument to `mcmanager.New`

```go
mgr, err := mcmanager.New(cfg, provider, mcmanager.Options{
    Scheme:                 scheme,
    Metrics:                metricsserver.Options{ /* ... */ },
    HealthProbeBindAddress: probeAddr,
    LeaderElection:         enableLeaderElection,
    LeaderElectionID:       "mcr-demo.example.com",
})
```

`mcmanager.Options` is a type alias for controller-runtime's `manager.Options`.

### 3c. The reconciler gets the Manager, not a client

The base `go/v4` plugin (which runs first in the chain) wants to inject a
single-cluster wiring — `Client: mgr.GetClient(), Scheme: mgr.GetScheme()` — but a
multicluster manager has neither method: its client is per-cluster. So the
`multicluster-runtime` plugin **rewrites that wiring** during `create api`, handing
the reconciler the manager instead:

```go
if err := (&controller.WidgetReconciler{
    Manager: mgr,
}).SetupWithManager(mgr); err != nil {
```

That rewrite is the last piece that lets the generated project build with no edits:

```bash
make generate manifests
go build ./...    # exit 0
```

> **Takeaway.** The plugin encodes the three non-obvious bits of multicluster
> wiring — scheme injection, provider-as-second-argument, and
> manager-not-client — so a fresh scaffold compiles as-is. (True for the
> `kubeconfig` provider used here; the other providers are still catching up —
> see *Switching providers* below.)

---

## Step 4 — The controller

The plugin also scaffolded a **compiling controller skeleton**: it already holds
the `Manager` and resolves the per-cluster client via
`r.Manager.GetCluster(ctx, req.ClusterName)`, with a `TODO` for your logic. Let's
give `Widget` real fields and fill that in.

Give `Widget` a spec/status worth reconciling (`api/v1/widget_types.go`):

```go
type WidgetSpec struct {
    // message is copied into a ConfigMap in the same cluster as the Widget.
    // +optional
    Message string `json:"message,omitempty"`
}

type WidgetStatus struct {
    // +optional
    ObservedCluster string `json:"observedCluster,omitempty"`
    // +optional
    ConfigMapName string `json:"configMapName,omitempty"`
}
```

Now the reconciler (`internal/controller/widget_controller.go`). Note it holds
the **`Manager`**, resolves the per‑cluster client from `req.ClusterName`, and
writes back into that *same* cluster:

```go
type WidgetReconciler struct {
    Manager mcmanager.Manager
}

func (r *WidgetReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
    log := logf.FromContext(ctx).WithValues("cluster", req.ClusterName)

    // The client for the cluster this event came from.
    cl, err := r.Manager.GetCluster(ctx, req.ClusterName)
    if err != nil {
        return ctrl.Result{}, fmt.Errorf("getting cluster %q: %w", req.ClusterName, err)
    }
    c := cl.GetClient()

    var widget demov1.Widget
    if err := c.Get(ctx, req.NamespacedName, &widget); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Mirror the Widget into a ConfigMap that lives in the SAME cluster.
    cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
        Name: "widget-" + widget.Name, Namespace: widget.Namespace,
    }}
    op, err := controllerutil.CreateOrUpdate(ctx, c, cm, func() error {
        if cm.Data == nil {
            cm.Data = map[string]string{}
        }
        cm.Data["cluster"] = string(req.ClusterName)
        cm.Data["widget"] = widget.Name
        cm.Data["message"] = widget.Spec.Message
        return controllerutil.SetControllerReference(&widget, cm, cl.GetScheme())
    })
    if err != nil {
        return ctrl.Result{}, err
    }

    widget.Status.ObservedCluster = string(req.ClusterName)
    widget.Status.ConfigMapName = cm.Name
    if err := c.Status().Update(ctx, &widget); err != nil {
        return ctrl.Result{}, err
    }

    log.Info("reconciled Widget", "op", op, "configmap", cm.Name)
    return ctrl.Result{}, nil
}

func (r *WidgetReconciler) SetupWithManager(mgr mcmanager.Manager) error {
    return mcbuilder.ControllerManagedBy(mgr).
        For(&demov1.Widget{}).
        Owns(&corev1.ConfigMap{}).
        Named("widget").
        Complete(r)
}
```

Regenerate and build:

```bash
make generate manifests
go build -o bin/manager ./cmd    # compiles clean against v0.24.1
```

---

## Step 5 — Two clusters, and how they register

### Why the operator runs on the host

`kind` publishes each cluster's API server on `https://127.0.0.1:<port>`. That's
reachable from **your host**, but *not* from inside a pod on another cluster. So
for a local demo the simplest, most reliable topology is to run the operator as a
plain process on your machine. It connects to cluster **A** for its own manager
and to watch the kubeconfig `Secret`s; the member clusters are reached via the
kubeconfigs inside those Secrets.

```text
┌─────────────────────────── your host ───────────────────────────┐
│  ./bin/manager  (KUBECONFIG = mcr-a)                             │
│     • manager + Secret watch  ── connects to ── mcr-a            │
│     • member clients          ── from Secrets ─ mcr-a, mcr-b     │
└─────┬──────────────────────────────────┬────────────────────────┘
   127.0.0.1:PORT_A                    127.0.0.1:PORT_B
   ┌──▼── kind: mcr-a ──┐            ┌──▼── kind: mcr-b ──┐
   │ Widget → ConfigMap │            │ Widget → ConfigMap │
   └────────────────────┘            └────────────────────┘
```

*(In production you'd instead deploy the operator into a hub cluster and use
kubeconfigs whose server addresses are reachable cluster‑to‑cluster.)*

### The registration model

The kubeconfig provider watches `Secret`s and turns each one into an engaged
cluster. The rules (all verifiable in the provider source):

- The Secret must carry the label **`sigs.k8s.io/multicluster-runtime-kubeconfig: "true"`**
  (the value is literally `"true"`).
- The kubeconfig lives under the data key **`kubeconfig`**.
- The **Secret's name becomes the cluster name** — i.e. `req.ClusterName`.

So registering our two clusters is just: install the CRD on both, then drop two
labeled Secrets into cluster A. That's exactly what [`demo/kind-up.sh`](https://github.com/v47/multi-cluster-plugin-demo/blob/HEAD/demo/kind-up.sh)
does; its core is:

```bash
# install the CRD on both clusters
kubectl --context kind-mcr-a apply -f config/crd/bases/demo.example.com_widgets.yaml
kubectl --context kind-mcr-b apply -f config/crd/bases/demo.example.com_widgets.yaml

# register each cluster as a labeled Secret in cluster A (Secret name = cluster name)
for c in mcr-a mcr-b; do
  kind get kubeconfig --name "$c" > ".work/$c.kubeconfig"
  kubectl --context kind-mcr-a -n default create secret generic "$c" \
    --from-file=kubeconfig=".work/$c.kubeconfig"
  kubectl --context kind-mcr-a -n default label secret "$c" \
    sigs.k8s.io/multicluster-runtime-kubeconfig=true
done
```

Note cluster **A registers itself too** — it's both the registry (it holds the
Secrets) and a member.

---

## Step 6 — Run it

Three commands (the demo scripts wrap exactly what's above):

```bash
./demo/kind-up.sh        # 2 kind clusters + CRDs + register both members
./demo/run-operator.sh   # build + run the operator on the host (leave running)
./demo/demo.sh           # create a Widget in each cluster and verify
```

When the operator starts, the provider engages both clusters:

```text
INFO  kubeconfig-provider  Successfully engaged manager  {"cluster": "mcr-a", "secret": "default/mcr-a"}
INFO  kubeconfig-provider  Successfully engaged manager  {"cluster": "mcr-b", "secret": "default/mcr-b"}
INFO  setup                starting manager
```

*(You'll also see a one‑off `optionsError: json: unsupported type: cluster.Option`
line at startup. It's benign — the provider just can't JSON‑log our
function‑typed `ClusterOptions`; engagement still succeeds.)*

`demo.sh` creates `Widget/hello` in each cluster and waits for the operator to
report back. The reconcile log shows the **same controller firing with different
cluster names**:

```text
INFO  reconciled Widget  {"controller": "widget", "cluster": "mcr-a", "op": "created", "configmap": "widget-hello"}
INFO  reconciled Widget  {"controller": "widget", "cluster": "mcr-b", "op": "created", "configmap": "widget-hello"}
```

And the proof — each cluster now has a `ConfigMap` the operator wrote, stamped
with the cluster it belongs to:

```text
==> Widgets (note the Observed-Cluster column differs per cluster)
--- mcr-a ---
NAME    MESSAGE         OBSERVED-CLUSTER
hello   hi from mcr-a   mcr-a
--- mcr-b ---
NAME    MESSAGE         OBSERVED-CLUSTER
hello   hi from mcr-b   mcr-b

==> ConfigMaps the single operator wrote into each cluster
--- mcr-a --- {"cluster":"mcr-a","message":"hi from mcr-a","widget":"hello"}
--- mcr-b --- {"cluster":"mcr-b","message":"hi from mcr-b","widget":"hello"}
```

One process. Two clusters. `req.ClusterName` routed every read, write and status
update to the correct one.

---

## Bonus: it behaves like a real controller

**Owner references work across the fleet.** We set the `Widget` as the owner of
its `ConfigMap`, so ordinary garbage collection applies — in the right cluster:

```bash
$ kubectl --context kind-mcr-b delete widget hello
widget.demo.example.com "hello" deleted
$ kubectl --context kind-mcr-b wait --for=delete configmap/widget-hello --timeout=30s
# GC confirmed: the ConfigMap is removed with its Widget — and mcr-a is untouched
```

**The fleet is dynamic.** Because clusters are just labeled Secrets, you add or
remove one at runtime with no operator restart. The snippet below is illustrative
(same registration mechanism as the two clusters above):

```bash
# add a third cluster
kind create cluster --name mcr-c
kubectl --context kind-mcr-c apply -f config/crd/bases/demo.example.com_widgets.yaml
kind get kubeconfig --name mcr-c > .work/mcr-c.kubeconfig
kubectl --context kind-mcr-a -n default create secret generic mcr-c \
  --from-file=kubeconfig=.work/mcr-c.kubeconfig
kubectl --context kind-mcr-a -n default label secret mcr-c \
  sigs.k8s.io/multicluster-runtime-kubeconfig=true
# the operator engages mcr-c within seconds; delete the Secret to drop it again
```

---

## Switching providers

The plugin records your provider choice and can rewrite `cmd/main.go` for a
different one:

```bash
kubebuilder edit --plugins multicluster-runtime.sigs.k8s/v1-alpha --provider namespace
```

The `namespace` provider treats each namespace as a cluster (great for a
single‑cluster demo); `file` reads a directory of kubeconfigs (great for CI);
`cluster-api` discovers clusters that Cluster API manages. Same reconciler, same
`req.ClusterName` — only discovery changes.

All four providers now generate **compiling** code against v0.24.1. `kubeconfig`, `namespace`, and `file` are verified end-to-end against local kind clusters; `cluster-api` compiles but needs a real Cluster API management cluster to exercise (and pulls `sigs.k8s.io/cluster-api` into your module). Note that `file` and `cluster-api` ship as **separate nested modules** (e.g. `sigs.k8s.io/multicluster-runtime/providers/cluster-api`); `go mod tidy` resolves them for you when you select that provider.

---

## Honest notes

- **Alpha plugin, pre-1.0 library.** All four provider scaffolds compile against
  **v0.24.1** out of the box (`kubeconfig`, `namespace`, `file` are also verified
  running against kind). multicluster-runtime is still pre-1.0, though: a newer
  release may move `mcmanager.New` or the provider options again, so pin the
  version you scaffold against.
- **Inject your scheme into `ClusterOptions`.** It's the easiest thing to forget
  and the failure mode ("no kind is registered for Widget") is confusing.
- **Operator‑on‑host is a local convenience.** It sidesteps `kind`'s
  `127.0.0.1` API endpoints. In production, run in‑cluster with kubeconfigs whose
  servers are reachable across clusters, and give the operator real RBAC on each
  member (here every kubeconfig is `kind`'s cluster‑admin).

---

## Wrap‑up

multicluster-runtime takes the single best idea in controller‑runtime — the
manager/reconciler model — and adds one dimension: **which cluster**. Kubebuilder's
plugin gets you a project that already speaks that dialect, and once you've wired
it to a pinned release, a fleet‑wide controller is barely more code than a
single‑cluster one. The reconciler you write is almost identical; the framework
fans it out across every cluster your provider engages.

Clone this repo, run the three scripts, and watch one binary reconcile two
clusters. Then delete a Secret, or add a cluster, and watch the fleet change
under a controller that never restarted.

### Links

- multicluster-runtime — https://github.com/kubernetes-sigs/multicluster-runtime
- Kubebuilder — https://book.kubebuilder.io/
- kind — https://kind.sigs.k8s.io/
- The demo in this repo — [`demo/`](https://github.com/v47/multi-cluster-plugin-demo/tree/HEAD/demo)

*Teardown when you're done:* `./demo/kind-down.sh`
