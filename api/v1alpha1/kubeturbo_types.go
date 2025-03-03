/*
Copyright 2024.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NB - if a block is marked as omitempty, it must be given a default value in order for inner fields to be populated with their defaults
// a single inner field value will suffice

// +kubebuilder:pruning:PreserveUnknownFields
// KubeturboSpec defines the desired state of Kubeturbo
type KubeturboSpec struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:pruning:PreserveUnknownFields
// KubeturboStatus defines the observed state of Kubeturbo
type KubeturboStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:path=kubeturbos,shortName=kt
//+kubebuilder:subresource:status

// Kubeturbo is the Schema for the kubeturbos API
type Kubeturbo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeturboSpec   `json:"spec"`
	Status KubeturboStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KubeturboList contains a list of Kubeturbo
type KubeturboList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kubeturbo `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kubeturbo{}, &KubeturboList{})
}
