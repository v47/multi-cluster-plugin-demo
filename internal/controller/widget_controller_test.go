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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	demov1 "github.com/v47/mcr-demo/api/v1"
)

var _ = Describe("Widget Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		widget := &demov1.Widget{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Widget")
			err := k8sClient.Get(ctx, typeNamespacedName, widget)
			if err != nil && errors.IsNotFound(err) {
				resource := &demov1.Widget{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: demov1.WidgetSpec{Message: "hello"},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &demov1.Widget{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Widget")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should persist the Widget resource", func() {
			// WidgetReconciler.Reconcile resolves a per-cluster client from
			// req.ClusterName and therefore needs a running mcmanager.Manager. That
			// multicluster path is exercised end-to-end against the two kind
			// clusters (see scripts/ and the article), not in this single-cluster
			// envtest. Here we just assert the API type round-trips.
			By("reading the created resource back")
			created := &demov1.Widget{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, created)).To(Succeed())
			Expect(created.Spec.Message).To(Equal("hello"))
		})
	})
})
