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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fwdeploy
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Progressing",type=integer,JSONPath=`.status.progressingReplicas`
// +kubebuilder:printcolumn:name="Unhealthy",type=integer,JSONPath=`.status.unhealthyReplicas`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
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
	// StrategyRecreate removes the old firewall and then creates a new one
	StrategyRecreate = "Recreate"
)

type FirewallDeploymentSpec struct {
	Strategy FirewallUpdateStrategy `json:"strategy"`
	Replicas int                    `json:"replicas"`
	Template FirewallSpec           `json:"template"`
}

type FirewallDeploymentStatus struct {
	ProgressingReplicas int `json:"progressingReplicas"`
	ReadyReplicas       int `json:"readyReplicas"`
	UnhealthyReplicas   int `json:"unhealthyReplicas"`
}

func (fl *FirewallDeploymentList) Validate() error {
	for _, f := range fl.Items {
		return f.Validate()
	}

	return nil
}

func (f *FirewallDeployment) Validate() error {
	if f.Spec.Replicas > 1 {
		return fmt.Errorf("for now, no more than a single firewall replica is allowed")
	}
	if f.Spec.Strategy != StrategyRecreate && f.Spec.Strategy != StrategyRollingUpdate {
		return fmt.Errorf("unknown strategy: %s", f.Spec.Strategy)
	}

	if f.Spec.Template.Name != "" {
		return fmt.Errorf("name will be set by the controller, cannot be set by the user")
	}
	if f.Spec.Template.Userdata != "" {
		return fmt.Errorf("userdata will be set by the controller, cannot be set by the user")
	}

	return nil
}
