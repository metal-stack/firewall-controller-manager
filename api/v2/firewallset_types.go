/*


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

package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FirewallSetList contains a set of FirewallSets
// +kubebuilder:object:root=true
type FirewallSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FirewallSet `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fwset
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Progressing",type=integer,JSONPath=`.status.progressingReplicas`
// +kubebuilder:printcolumn:name="Unhealthy",type=integer,JSONPath=`.status.unhealthyReplicas`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type FirewallSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec     FirewallSetSpec `json:"spec,omitempty"`
	Userdata string          `json:"userdata"`

	Status FirewallSetStatus `json:"status,omitempty"`
}

type FirewallSetSpec struct {
	Replicas int          `json:"replicas"`
	Template FirewallSpec `json:"template"`
}

type FirewallSetStatus struct {
	ProgressingReplicas int `json:"progressingReplicas"`
	ReadyReplicas       int `json:"readyReplicas"`
	UnhealthyReplicas   int `json:"unhealthyReplicas"`
}

func (f *FirewallSet) Validate() error {
	return nil
}

func (f *FirewallSetList) GetItems() []*FirewallSet {
	var result []*FirewallSet
	for i := range f.Items {
		result = append(result, &f.Items[i])
	}
	return result
}
