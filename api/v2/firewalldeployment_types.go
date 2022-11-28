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

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fwdeploy
// +kubebuilder:subresource:status
type FirewallDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FirewallDeploymentSpec   `json:"spec,omitempty"`
	Status FirewallDeploymentStatus `json:"status,omitempty"`
}

// FirewallDeploymentList contains a list of FirewallDeployments
// +kubebuilder:object:root=true
type FirewallDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FirewallDeployment `json:"items"`
}

type FirewallUpdateStrategy string

const (
	// StrategyRollingUpdate first creates a new firewall, wait's until it is ready and then removes the old one
	StrategyRollingUpdate = "RollingUpdate"
	// StrategyReplace removes the old firewall and then creates a new one
	StrategyReplace = "Replace"
)

type FirewallDeploymentSpec struct {
	Strategy FirewallUpdateStrategy `json:"strategy"`
	Replicas int                    `json:"replicas"`
	Template Firewall               `json:"template"`
}

type FirewallDeploymentStatus struct {
	Reconciled  bool     `json:"reconciled"`
	FirewallIDs []string `json:"firewallIDs"`
}
