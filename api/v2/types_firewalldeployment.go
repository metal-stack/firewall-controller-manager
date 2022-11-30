package v2

import (
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
	TargetReplicas      int        `json:"targetReplicas"`
	ProgressingReplicas int        `json:"progressingReplicas"`
	ReadyReplicas       int        `json:"readyReplicas"`
	UnhealthyReplicas   int        `json:"unhealthyReplicas"`
	ObservedRevision    int        `json:"observedRevision"`
	Conditions          Conditions `json:"conditions"`
}

const (
	// FirewallDeplomentAvailable indicates whether the deployment has reached the desired amount of replicas or not.
	FirewallDeplomentAvailable ConditionType = "Available"
	// FirewallDeplomentAvailable indicates whether the deployment has reached the desired amount of replicas or not.
	FirewallDeplomentProgressing ConditionType = "Progressing"
	// FirewallDeplomentRBACProvisioned indicates whether the rbac permissions for the firewall-controller to communicate with the api server were provisioned.
	FirewallDeplomentRBACProvisioned ConditionType = "RBACProvisioned"
	// FirewallDeplomentEgressIPs indicates whether the egress ips of the firewall are properly configured and annotated with ip tags.
	FirewallDeplomentEgressIPs ConditionType = "EgressIPs"
)
