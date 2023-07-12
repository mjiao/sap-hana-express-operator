/*
Copyright 2023.

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

// Credential contains the credential information intended to be used
type Credential struct {
	// +kubebuilder:validation:Required
	// The Name defines the predefined Kubernetes secret name for initilizing the users of the Hana Express DB
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	// The key of the secret to select from.  Must be a valid secret key
	Key string `json:"key"`
}

// HanaExpressSpec defines the desired state of HanaExpress
type HanaExpressSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^\d+Gi$`
	// PVCSize defines the Persistent volume size attached to the Hana Express StatefulSet
	PVCSize string `json:"pvcSize"`

	// +kubebuilder:validation:Required
	Credential Credential `json:"credential"`
}

// HanaExpressStatus defines the observed state of HanaExpress
type HanaExpressStatus struct {
	// Represents the observations of a HanaExpress's current state.
	// HanaExpress.status.conditions.type are: "Available", "Progressing", and "Degraded"
	// HanaExpress.status.conditions.status are one of True, False, Unknown.
	// HanaExpress.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// HanaExpress.status.conditions.Message is a human-readable message indicating details about the transition.

	// Conditions store the status conditions of the HanaExpress instances
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HanaExpress is the Schema for the hanaexpresses API
type HanaExpress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HanaExpressSpec   `json:"spec,omitempty"`
	Status HanaExpressStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HanaExpressList contains a list of HanaExpress
type HanaExpressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HanaExpress `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HanaExpress{}, &HanaExpressList{})
}
