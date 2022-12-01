package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FirewallSet contains the spec template of a firewall resource similar to a Kubernetes ReplicaSet and takes care that the desired amount of firewall replicas is running.
//
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

	// Spec contains the firewall set specification.
	Spec FirewallSetSpec `json:"spec,omitempty"`
	// Userdata contains the userdata used for the creation of the firewall.
	// It is not part of the template as it is generated dynamically by a controller that governs the firewall.
	Userdata string `json:"userdata"`
	// Status contains current status information on the firewall set.
	Status FirewallSetStatus `json:"status,omitempty"`
}

// FirewallSetSpec specifies the firewall set.
type FirewallSetSpec struct {
	// Replicas is the amount of firewall replicas targeted to be running.
	Replicas int `json:"replicas"`
	// Template is the firewall spec used for creating the firewalls.
	Template FirewallSpec `json:"template"`
}

type FirewallSetStatus struct {
	// TargetReplicas is the amount of firewall replicas targeted to be running.
	TargetReplicas int `json:"targetReplicas"`
	// ProgressingReplicas is the amount of firewall replicas that are currently progressing in the latest managed firewall set.
	ProgressingReplicas int `json:"progressingReplicas"`
	// ProgressingReplicas is the amount of firewall replicas that are currently ready in the latest managed firewall set.
	ReadyReplicas int `json:"readyReplicas"`
	// ProgressingReplicas is the amount of firewall replicas that are currently unhealthy in the latest managed firewall set.
	UnhealthyReplicas int `json:"unhealthyReplicas"`
	// ObservedRevision is a counter that increases with each firewall set roll that was made.
	ObservedRevision int `json:"observedRevision"`
}

// FirewallSetList contains a list of firewalls sets
//
// +kubebuilder:object:root=true
type FirewallSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items contains the list items.
	Items []FirewallSet `json:"items"`
}

func (f *FirewallSetList) GetItems() []*FirewallSet {
	var result []*FirewallSet
	for i := range f.Items {
		result = append(result, &f.Items[i])
	}
	return result
}
