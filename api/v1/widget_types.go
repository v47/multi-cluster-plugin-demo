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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WidgetSpec defines the desired state of Widget.
type WidgetSpec struct {
	// message is copied by the operator into a ConfigMap that lives in the same
	// cluster as this Widget.
	// +optional
	Message string `json:"message,omitempty"`
}

// WidgetStatus defines the observed state of Widget.
type WidgetStatus struct {
	// observedCluster is the name of the cluster in which the operator last
	// reconciled this Widget. With a single-cluster manager this would always be
	// empty; with multicluster-runtime it is the provider's cluster name.
	// +optional
	ObservedCluster string `json:"observedCluster,omitempty"`

	// configMapName is the ConfigMap the operator created/updated in that cluster.
	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.spec.message`
// +kubebuilder:printcolumn:name="Observed-Cluster",type=string,JSONPath=`.status.observedCluster`

// Widget is the Schema for the widgets API
type Widget struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Widget
	// +required
	Spec WidgetSpec `json:"spec"`

	// status defines the observed state of Widget
	// +optional
	Status WidgetStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// WidgetList contains a list of Widget
type WidgetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Widget `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Widget{}, &WidgetList{})
		return nil
	})
}
