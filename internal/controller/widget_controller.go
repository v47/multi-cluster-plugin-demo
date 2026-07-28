/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	demov1 "github.com/v47/mcr-demo/api/v1"
)

// WidgetReconciler reconciles Widget objects across every cluster registered
// with the multicluster provider.
//
// Unlike a single-cluster controller-runtime reconciler, it does NOT embed a
// client.Client: there is no single "the" cluster. Instead it holds the
// multicluster Manager and resolves the right client per request using
// req.ClusterName.
type WidgetReconciler struct {
	Manager mcmanager.Manager
}

// +kubebuilder:rbac:groups=demo.example.com,resources=widgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=demo.example.com,resources=widgets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=demo.example.com,resources=widgets/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles Widget objects across all clusters managed by the
// multicluster provider. req.ClusterName identifies which cluster the event
// originated from; every client operation below targets that same cluster.
func (r *WidgetReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("cluster", req.ClusterName)

	// Resolve the client for the cluster this event came from.
	cl, err := r.Manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting cluster %q: %w", req.ClusterName, err)
	}
	c := cl.GetClient()

	var widget demov1.Widget
	if err := c.Get(ctx, req.NamespacedName, &widget); err != nil {
		// Ignore not-found: the Widget was deleted; its owned ConfigMap is
		// garbage-collected by the owner reference set below.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Mirror the Widget into a ConfigMap that lives in the SAME cluster. This is
	// the visible proof of multicluster reconciliation: a single operator process
	// writes into whichever member cluster the Widget appeared in.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "widget-" + widget.Name,
			Namespace: widget.Namespace,
		},
	}
	op, err := controllerutil.CreateOrUpdate(ctx, c, cm, func() error {
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data["cluster"] = string(req.ClusterName)
		cm.Data["widget"] = widget.Name
		cm.Data["message"] = widget.Spec.Message
		// Owner reference so the ConfigMap is cleaned up with the Widget. The
		// scheme comes from the member cluster (populated via ClusterOptions in
		// cmd/main.go) so it knows the Widget GVK.
		return controllerutil.SetControllerReference(&widget, cm, cl.GetScheme())
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("upserting configmap in cluster %q: %w", req.ClusterName, err)
	}

	// Record what happened on the Widget's status, in that same cluster.
	widget.Status.ObservedCluster = string(req.ClusterName)
	widget.Status.ConfigMapName = cm.Name
	if err := c.Status().Update(ctx, &widget); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating widget status in cluster %q: %w", req.ClusterName, err)
	}

	log.Info("reconciled Widget",
		"op", op, "configmap", cm.Name, "message", widget.Spec.Message)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the multicluster Manager. The
// mcbuilder wires the same controller to every cluster the provider engages.
func (r *WidgetReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&demov1.Widget{}).
		Owns(&corev1.ConfigMap{}).
		Named("widget").
		Complete(r)
}
